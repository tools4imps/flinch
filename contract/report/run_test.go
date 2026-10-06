package report_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/report"
)

// copyFixture copies a fixture module into a temp directory and returns its path.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// flinch runs flinch in the current directory and returns its exit code, stdout and stderr.
func flinch(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := cli.Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// Contract: report/P6
func TestProgressGoesToStderrAndTheReportToStdoutOrAFile(t *testing.T) {
	t.Chdir(copyFixture(t, "testdata/mod"))

	code, out, errs := flinch("contract", "--format", "json")
	var v struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.SchemaVersion != report.SchemaVersion || code != 0 || errs != "" {
		t.Errorf("flinch contract --format json exited %d with stderr %q and stdout\n%s", code, errs, out)
	}

	code, out, errs = flinch("--jobs", "2", "--output", "report.txt")
	if code != 1 {
		t.Errorf("the run exited %d, want 1 for the fixture's unheld mutants\n%s", code, errs)
	}
	if out != "" {
		t.Errorf("with --output, stdout should stay empty, and it holds:\n%s", out)
	}
	if !strings.Contains(errs, "flinch: running") {
		t.Errorf("stderr shows no progress:\n%s", errs)
	}
	for _, l := range strings.Split(strings.TrimSuffix(errs, "\n"), "\n") {
		if !strings.HasPrefix(l, "flinch: ") {
			t.Errorf("stderr holds more than progress: %q", l)
		}
	}
	data, err := os.ReadFile("report.txt")
	if err != nil {
		t.Fatal(err)
	}
	rep := string(data)
	if !strings.HasPrefix(rep, "flinch: 4 mutants in covered code\n") || !strings.Contains(rep, "\nUnheld (3)\n") ||
		!strings.HasSuffix(rep, "\nexit 1\n") {
		t.Errorf("the report file holds:\n%s", rep)
	}
}
