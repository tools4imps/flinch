package runner

import (
	"sort"
	"strconv"
	"strings"
)

// The testing package, run with -test.v=test2json, starts every framing line with this byte. Reading the
// framed stream directly saves a process per run, and it lets a crashed run keep the result of the test
// that panicked: go tool test2json holds a "--- FAIL" report back until more framing arrives, so it
// drops that report when the process dies right after printing it.
const marker = "\x16"

// A stream is what one test process printed, read the way go tool test2json reads it.
type stream struct {
	started map[string]bool     // full test names with a RUN line
	results map[string]string   // "pass", "fail" or "skip", by full test name
	elapsed map[string]float64  // seconds, from the report line
	output  map[string][]string // lines that aren't framing, by the test they belong to ("" for none)
	ended   bool                // the final PASS or FAIL line arrived, so the process finished normally

	// timedOut lists the top-level tests in the "running tests:" block of a timeout panic. The block is
	// the only reliable account of what was running: the stream attributes the panic text to whichever
	// test spoke last, and with t.Parallel that's often a test that already finished.
	timedOut []string
	timeout  bool
}

var updates = []string{"=== RUN   ", "=== PAUSE ", "=== CONT  ", "=== NAME  "}

var reports = []struct{ prefix, result string }{
	{"--- PASS: ", "pass"},
	{"--- FAIL: ", "fail"},
	{"--- SKIP: ", "skip"},
}

func parseStream(out []byte) *stream {
	s := &stream{
		started: map[string]bool{},
		results: map[string]string{},
		elapsed: map[string]float64{},
		output:  map[string][]string{},
	}
	cur := ""
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		// A test that prints without a newline leaves the next framing line mid-line, so each marker
		// starts a new line wherever it falls, as it does for test2json.
		parts := strings.Split(line, marker)
		if parts[0] != "" || len(parts) == 1 {
			s.output[cur] = append(s.output[cur], parts[0])
		}
		for _, frame := range parts[1:] {
			cur = s.frame(frame, cur)
		}
		if strings.HasPrefix(line, "panic: test timed out after ") {
			s.timeout = true
			s.timedOut = runningTests(lines[i+1:])
		}
	}
	return s
}

// frame records one framing line and returns the test that output after it belongs to.
func (s *stream) frame(line, cur string) string {
	if line == "PASS" || line == "FAIL" {
		s.ended = true
		return ""
	}
	for _, u := range updates {
		if name, ok := strings.CutPrefix(line, u); ok {
			name = strings.TrimSpace(name)
			if u == "=== RUN   " {
				s.started[name] = true
			}
			return name
		}
	}
	line = strings.TrimLeft(line, " ")
	for _, r := range reports {
		rest, ok := strings.CutPrefix(line, r.prefix)
		if !ok {
			continue
		}
		name := strings.TrimSpace(rest)
		if i := strings.LastIndex(name, " ("); i >= 0 && strings.HasSuffix(name, "s)") {
			if sec, err := strconv.ParseFloat(name[i+2:len(name)-2], 64); err == nil {
				s.elapsed[name[:i]] = sec
			}
			name = name[:i]
		}
		s.results[name] = r.result
		return name
	}
	return cur
}

// runningTests reads the tests a timeout panic lists, which follow it as "\trunning tests:" and then one
// "\t\tName (2s)" line each, and returns their top-level names.
func runningTests(lines []string) []string {
	seen := map[string]bool{}
	in := false
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if !in {
			if strings.TrimSpace(line) == "running tests:" {
				in = true
				continue
			}
			if strings.TrimSpace(line) != "" {
				break
			}
			continue
		}
		name, ok := strings.CutPrefix(line, "\t\t")
		if !ok {
			break
		}
		if i := strings.LastIndex(name, " ("); i >= 0 {
			name = name[:i]
		}
		seen[top(name)] = true
	}
	return sortedKeys(seen)
}

// top returns the top-level test a full test name belongs to.
func top(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

func (s *stream) anyStarted() bool { return len(s.started) > 0 }

// failedSubtests lists the subtests of t that failed, by their full names.
func (s *stream) failedSubtests(t string) []string {
	var out []string
	for name, r := range s.results {
		if r == "fail" && strings.HasPrefix(name, t+"/") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// panicked reports whether the output that belongs to t, or to one of its subtests, holds a panic. A
// panicking test prints its "--- FAIL" report before the runtime prints the panic, so the panic text
// follows that report in the stream.
func (s *stream) panicked(t string) bool {
	for name, lines := range s.output {
		if name != t && !strings.HasPrefix(name, t+"/") {
			continue
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "panic: ") && !strings.HasPrefix(l, "panic: test timed out after ") {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
