// Package since finds what changed between a ref and the working tree, for runs scoped with --since.
// It asks git, and it pins every option that could change the answer, so a user's git config, a diff
// driver or an environment variable can't move which lines count as changed.
package since

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Changes is what changed since the merge base: the lines on the new side of each file, and every
// path the change touched.
type Changes struct {
	Base  string            // the merge base commit
	lines map[string][]span // new-side changed lines, by module-relative path
	paths map[string]bool   // every changed path, old and new sides of a rename, deletions included
}

// A span is a run of changed lines, first and last included.
type span struct{ first, last int }

// Changed finds the merge base of ref and HEAD and diffs the working tree against it. Untracked files
// that git doesn't ignore count as new, or as the new side of a rename, because the Go toolchain
// builds them all the same.
// Paths come back relative to root, which may sit below the top of the repository; changes outside
// root are left out.
func Changed(ctx context.Context, root, ref string) (*Changes, error) {
	if ref == "" {
		return nil, errors.New("--since needs a ref")
	}
	out, err := git(ctx, root, "merge-base", ref, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("finding the merge base of %s and HEAD: %w", ref, err)
	}
	base := strings.TrimSpace(string(out))
	if base == "" {
		return nil, fmt.Errorf("%s and HEAD have no merge base", ref)
	}
	out, err = git(ctx, root, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("finding where %s sits in its repository: %w", root, err)
	}
	prefix := strings.TrimSpace(string(out))

	c := &Changes{
		Base:  base,
		lines: map[string][]span{},
		paths: map[string]bool{},
	}

	// Untracked files that git doesn't ignore count, because the Go toolchain builds them all the
	// same. They go into a copy of the index as intent-to-add entries, so the diffs below see each
	// one as a new file and can pair it with a deleted file as a rename, whether or not the user
	// staged the move. The repository's own index is never written.
	work, err := os.MkdirTemp("", "flinch-since-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	index, err := copyIndex(ctx, root, work)
	if err != nil {
		return nil, err
	}
	env := []string{"GIT_INDEX_FILE=" + index}

	// ls-files reports paths relative to the directory it runs in, and only the ones below it, so
	// these need no prefix stripped. The global excludes file is pinned to nothing, because it's
	// the one ignore list that lives on the machine and not in the repository.
	out, err = gitWith(ctx, root, env, nil, "-c", "core.excludesFile="+os.DevNull, "-c", "core.quotePath=false",
		"ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("listing untracked files: %w", err)
	}
	var untracked []string
	for _, name := range strings.Split(string(out), "\x00") {
		// An if chain, not a switch: flinch can't see a test reach a switch's case expressions.
		if strings.HasSuffix(name, "/") {
			// A nested repository, which git lists whole and can't add, so only its path counts.
			c.paths[name] = true
		} else if name != "" {
			untracked = append(untracked, name)
		}
	}
	if len(untracked) > 0 {
		_, err = gitWith(ctx, root, env, strings.NewReader(strings.Join(untracked, "\x00")),
			"--literal-pathspecs", "add", "--force", "--intent-to-add", "--pathspec-from-file=-", "--pathspec-file-nul")
		if err != nil {
			return nil, fmt.Errorf("noting untracked files in a copy of the index: %w", err)
		}
	}

	out, err = gitWith(ctx, root, env, nil, diffArgs(base, "--name-status", "-z")...)
	if err != nil {
		return nil, fmt.Errorf("listing the files changed since %s: %w", base, err)
	}
	names, err := parseNameStatus(out)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if p, ok := under(prefix, name); ok {
			c.paths[p] = true
		}
	}

	out, err = gitWith(ctx, root, env, nil, diffArgs(base, "--patch")...)
	if err != nil {
		return nil, fmt.Errorf("diffing the working tree against %s: %w", base, err)
	}
	hunks, err := parsePatch(out)
	if err != nil {
		return nil, err
	}
	for name, spans := range hunks {
		if p, ok := under(prefix, name); ok {
			c.lines[p] = append(c.lines[p], spans...)
		}
	}
	return c, nil
}

// copyIndex copies the repository's index into dir and returns the copy's path. With no index to
// copy, the path names a file that doesn't exist yet, which git reads as an empty index.
func copyIndex(ctx context.Context, root, dir string) (string, error) {
	out, err := git(ctx, root, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", fmt.Errorf("finding the git index: %w", err)
	}
	src := strings.TrimSpace(string(out))
	if !filepath.IsAbs(src) {
		src = filepath.Join(root, src)
	}
	dst := filepath.Join(dir, "index")
	data, err := os.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		return dst, nil
	}
	if err != nil {
		return "", fmt.Errorf("reading the git index: %w", err)
	}
	return dst, os.WriteFile(dst, data, 0o600)
}

// diffArgs builds a diff of the working tree against base with every option that changes hunks or
// paths set on the command line, where it beats any config file. --text reads every file as text,
// so no attributes file, the machine's own included, can turn a source file's hunks into a line
// saying the binary files differ.
func diffArgs(base string, format ...string) []string {
	args := []string{
		"-c", "core.quotePath=false",
		"-c", "diff.relative=false",
		"diff",
		"--no-color", "--no-ext-diff", "--no-textconv", "--text",
		"--unified=0", "--inter-hunk-context=0",
		"--find-renames=50%", "-l0",
		"--diff-algorithm=myers", "--indent-heuristic",
		"--ignore-submodules=dirty",
		"--src-prefix=a/", "--dst-prefix=b/",
	}
	args = append(args, format...)
	return append(args, base, "--")
}

// git runs one git command in dir with the environment variables that reach into diff output removed.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return gitWith(ctx, dir, nil, nil, args...)
}

// gitWith is git with env added to the environment and stdin, when it isn't nil, as the command's
// input.
func gitWith(ctx context.Context, dir string, env []string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(cleanEnv(os.Environ()), env...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %s", subcommand(args), msg)
		}
		return nil, err
	}
	return out, nil
}

// subcommand names the git command in args, past any "-c name=value" pairs, for error messages.
func subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// cleanEnv drops GIT_DIFF_OPTS, which can set the context size, and GIT_EXTERNAL_DIFF, which can
// replace git's own diff with any program.
func cleanEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_DIFF_OPTS=") || strings.HasPrefix(kv, "GIT_EXTERNAL_DIFF=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// under turns a path relative to the top of the repository into one relative to the module root,
// and says whether it sits inside the module at all.
func under(prefix, name string) (string, bool) {
	if prefix == "" {
		return name, true
	}
	if strings.HasPrefix(name, prefix) {
		return name[len(prefix):], true
	}
	return "", false
}

// parseNameStatus reads `git diff --name-status -z`: a status, then one path, or two for a rename
// or a copy, each ended by a NUL.
func parseNameStatus(out []byte) ([]string, error) {
	fields := strings.Split(string(out), "\x00")
	var names []string
	for i := 0; i < len(fields); i++ {
		status := fields[i]
		if status == "" {
			continue
		}
		n := 1
		if status[0] == 'R' || status[0] == 'C' {
			n = 2
		}
		if i+n >= len(fields) {
			return nil, fmt.Errorf("git diff --name-status ended in the middle of an entry for status %q", status)
		}
		names = append(names, fields[i+1:i+1+n]...)
		i += n
	}
	return names, nil
}

// parsePatch reads the hunks of a zero-context patch and returns the new-side lines each one
// changed, by the path on the new side. A hunk that only deletes lines changes no new-side line.
func parsePatch(out []byte) (map[string][]span, error) {
	hunks := map[string][]span{}
	file := ""
	// An added line holding "++ x" reads "+++ x", so a "+++ " line is a header only between
	// "diff --git" and the file's first hunk.
	header := false
	for _, line := range strings.Split(string(out), "\n") {
		// An if chain, not a switch: flinch can't see a test reach a switch's case expressions.
		if strings.HasPrefix(line, "diff --git ") {
			file, header = "", true
		} else if header && strings.HasPrefix(line, "+++ ") {
			name, err := headerPath(line[len("+++ "):])
			if err != nil {
				return nil, err
			}
			file = name
		} else if strings.HasPrefix(line, "@@ ") {
			header = false
			if file == "" {
				continue
			}
			s, ok, err := hunkSpan(line)
			if err != nil {
				return nil, err
			}
			if ok {
				hunks[file] = append(hunks[file], s)
			}
		}
	}
	return hunks, nil
}

// headerPath reads the path from a "+++ " header. It's "" for /dev/null, the new side of a
// deletion. Git quotes a path holding a tab, a newline, a quote or a backslash even with
// core.quotePath off, in the C style strconv.Unquote reads.
func headerPath(s string) (string, error) {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(s, `"`) {
		u, err := strconv.Unquote(s)
		if err != nil {
			return "", fmt.Errorf("reading the quoted path %s in git's diff: %w", s, err)
		}
		s = u
	}
	if !strings.HasPrefix(s, "b/") {
		return "", fmt.Errorf("git's diff named a new-side path without the b/ prefix: %s", s)
	}
	return s[2:], nil
}

// hunkSpan reads "@@ -a,b +c,d @@". When ",d" is missing the count is 1. A count of 0 means the hunk
// adds nothing on the new side.
func hunkSpan(line string) (span, bool, error) {
	rest := strings.TrimPrefix(line, "@@ ")
	end := strings.Index(rest, " @@")
	if end < 0 {
		return span{}, false, fmt.Errorf("git's diff has a hunk header without its closing @@: %s", line)
	}
	for _, f := range strings.Fields(rest[:end]) {
		if !strings.HasPrefix(f, "+") {
			continue
		}
		start, count := f[1:], "1"
		if i := strings.IndexByte(start, ','); i >= 0 {
			start, count = start[:i], start[i+1:]
		}
		c, err1 := strconv.Atoi(start)
		d, err2 := strconv.Atoi(count)
		if err1 != nil || err2 != nil {
			return span{}, false, fmt.Errorf("git's diff has a hunk header flinch can't read: %s", line)
		}
		if d == 0 {
			return span{}, false, nil
		}
		return span{c, c + d - 1}, true, nil
	}
	return span{}, false, fmt.Errorf("git's diff has a hunk header without a new side: %s", line)
}

// Touches reports whether line in file changed. Every line of a new file, untracked ones included,
// counts as changed.
func (c *Changes) Touches(file string, line int) bool {
	for _, s := range c.lines[file] {
		if s.first <= line && line <= s.last {
			return true
		}
	}
	return false
}

// TouchesDir reports whether any changed path sits in dir or below it. Dir "." holds every path.
func (c *Changes) TouchesDir(dir string) bool {
	dir = strings.TrimSuffix(dir, "/")
	if dir == "." || dir == "" {
		return len(c.paths) > 0
	}
	for p := range c.paths {
		if strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// Paths lists every changed path, sorted.
func (c *Changes) Paths() []string {
	ps := make([]string, 0, len(c.paths))
	for p := range c.paths {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps
}
