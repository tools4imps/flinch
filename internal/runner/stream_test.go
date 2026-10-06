package runner

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
)

// framed writes test output the way -test.v=test2json prints it, with "^" standing for the marker byte.
func framed(s string) []byte { return []byte(strings.ReplaceAll(s, "^", marker)) }

func TestParseStreamResults(t *testing.T) {
	s := parseStream(framed(`^=== RUN   TestA
    a_test.go:9: a log
^--- PASS: TestA (0.25s)
^=== NAME
^=== RUN   TestB
^=== RUN   TestB/x
    a_test.go:11: sub fails
^--- FAIL: TestB/x (0.00s)
^=== NAME  TestB
^=== RUN   TestB/y
^--- PASS: TestB/y (0.00s)
^=== NAME  TestB
^--- FAIL: TestB (0.01s)
^=== RUN   TestC
^--- SKIP: TestC (0.00s)
^FAIL
`))
	want := map[string]string{"TestA": "pass", "TestB": "fail", "TestB/x": "fail", "TestB/y": "pass", "TestC": "skip"}
	if !reflect.DeepEqual(s.results, want) {
		t.Errorf("results = %v, want %v", s.results, want)
	}
	if !s.ended {
		t.Error("the final FAIL line should end the stream")
	}
	if s.elapsed["TestA"] != 0.25 {
		t.Errorf("elapsed TestA = %v, want 0.25", s.elapsed["TestA"])
	}
	if got := s.failedSubtests("TestB"); !reflect.DeepEqual(got, []string{"TestB/x"}) {
		t.Errorf("failedSubtests = %v", got)
	}
	if got := s.output["TestB/x"]; !reflect.DeepEqual(got, []string{"    a_test.go:11: sub fails"}) {
		t.Errorf("TestB/x output = %q", got)
	}
	if s.panicked("TestB") || s.timeout {
		t.Error("nothing panicked or timed out")
	}
}

func TestParseStreamPanic(t *testing.T) {
	s := parseStream(framed(`^=== RUN   TestA
^--- PASS: TestA (0.00s)
^=== NAME
^=== RUN   TestSub
^=== RUN   TestSub/deep
^--- FAIL: TestSub/deep (0.00s)
^=== NAME  TestSub
^--- FAIL: TestSub (0.00s)
panic: boom [recovered, repanicked]

goroutine 7 [running]:
`))
	if s.ended {
		t.Error("a panic leaves no final line")
	}
	if s.results["TestSub"] != "fail" || !s.panicked("TestSub") {
		t.Errorf("TestSub should fail with a panic: %v", s.results)
	}
	if s.panicked("TestA") {
		t.Error("TestA didn't panic")
	}
}

// The timeout's panic text lands after TestParA's report, but the running tests block names TestParB.
func TestParseStreamParallelTimeout(t *testing.T) {
	s := parseStream(framed(`^=== RUN   TestParA
^=== PAUSE TestParA
^=== NAME
^=== RUN   TestParB
^=== PAUSE TestParB
^=== NAME
^=== CONT  TestParA
^=== CONT  TestParB
^--- PASS: TestParA (0.05s)
panic: test timed out after 1s
	running tests:
		TestParB (1s)
		TestParB/inner (1s)

goroutine 9 [running]:
`))
	if !s.timeout || !reflect.DeepEqual(s.timedOut, []string{"TestParB"}) {
		t.Errorf("timedOut = %v (timeout %v), want [TestParB]", s.timedOut, s.timeout)
	}
	if s.panicked("TestParA") {
		t.Error("a timeout isn't a panic of the test that spoke last")
	}
}

func TestParseStreamMarkerMidLine(t *testing.T) {
	s := parseStream(framed("^=== RUN   TestA\nno newline^--- PASS: TestA (0.00s)\n^=== RUN   TestB\n"))
	if s.results["TestA"] != "pass" || !s.started["TestB"] {
		t.Errorf("a marker mid-line should start a framing line: %v %v", s.results, s.started)
	}
	if got := s.output["TestA"]; !reflect.DeepEqual(got, []string{"no newline"}) {
		t.Errorf("TestA output = %q", got)
	}
}

func TestRunPattern(t *testing.T) {
	if got := runPattern([]string{"TestA", "ExampleB"}); got != "^(TestA|ExampleB)$" {
		t.Errorf("runPattern = %q", got)
	}
}

func proc0(raw string, guard bool) *proc {
	return &proc{stream: parseStream(framed(raw)), guard: guard}
}

func TestSettle(t *testing.T) {
	kill := func(k model.KillKind) *outcome { return &outcome{fail: true, kind: k} }
	pass := &outcome{}
	cases := []struct {
		name    string
		p       *proc
		set     []string
		want    map[string]*outcome
		settled int
		named   bool
	}{
		{"clean finish", proc0("^=== RUN   TestA\n^--- PASS: TestA (0.00s)\n^=== RUN   TestB\n    x_test.go:3: no\n^--- FAIL: TestB (0.00s)\n^FAIL\n", false),
			[]string{"TestA", "TestB"}, map[string]*outcome{"TestA": pass, "TestB": kill(model.Assertion)}, 2, true},
		{"init panic kills the whole set", proc0("panic: init\n\ngoroutine 1 [running]:\n", false),
			[]string{"TestA", "TestB"}, map[string]*outcome{"TestA": kill(model.Panic), "TestB": kill(model.Panic)}, 2, true},
		{"guard before any test is a timeout of the whole set", proc0("", true),
			[]string{"TestA"}, map[string]*outcome{"TestA": kill(model.Timeout)}, 1, true},
		{"guard after a test started names nobody", proc0("^=== RUN   TestA\n", true),
			[]string{"TestA"}, map[string]*outcome{}, 0, false},
		{"a lone test that dies is the killer", proc0("^=== RUN   TestA\nfatal error: stack overflow\n", false),
			[]string{"TestA"}, map[string]*outcome{"TestA": kill(model.Panic)}, 1, true},
		{"a crash among several names nobody", proc0("^=== RUN   TestA\n^--- PASS: TestA (0.00s)\n^=== RUN   TestB\n", false),
			[]string{"TestA", "TestB", "TestC"}, map[string]*outcome{"TestA": pass}, 1, false},
		{"a panic names its test", proc0("^=== RUN   TestA\n^--- FAIL: TestA (0.00s)\npanic: boom\n", false),
			[]string{"TestA", "TestB"}, map[string]*outcome{"TestA": kill(model.Panic)}, 1, true},
		{"a timeout names the running tests", proc0("^=== RUN   TestA\n^--- PASS: TestA (0.00s)\n^=== RUN   TestB\npanic: test timed out after 2s\n\trunning tests:\n\t\tTestB (2s)\n", false),
			[]string{"TestA", "TestB", "TestC"}, map[string]*outcome{"TestA": pass, "TestB": kill(model.Timeout)}, 2, true},
		{"a normal finish that skipped a test names nobody", proc0("^PASS\n", false),
			[]string{"TestA"}, map[string]*outcome{}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := map[string]*outcome{}
			n, named := settle(c.p, c.set, out)
			if n != c.settled || named != c.named {
				t.Errorf("settle = %d, %v; want %d, %v", n, named, c.settled, c.named)
			}
			if !reflect.DeepEqual(out, c.want) {
				t.Errorf("outcomes = %v, want %v", show(out), show(c.want))
			}
		})
	}
}

func show(m map[string]*outcome) map[string]outcome {
	out := map[string]outcome{}
	for k, v := range m {
		out[k] = *v
	}
	return out
}
