// Package gate turns a run's findings into an exit code. There is no score and no threshold: a run
// passes when nothing is wrong, fails when something is, and says it couldn't decide when it couldn't.
package gate

import (
	"fmt"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
)

// Exit codes.
const (
	Pass      = 0 // nothing fails the build
	Fail      = 1 // a Contract error, an unheld mutant, a broken declaration, a hollow obligation or a blind test
	Undecided = 2 // flinch couldn't decide; this wins over Fail
)

// A Verdict is the exit code and the reasons behind it, one line per kind of finding.
type Verdict struct {
	Exit    int
	Reasons []string
}

// Decide reads the findings and nothing else, so the job count, the machine and git config can't
// change it. Problems are Contract errors. The matrix is nil when the run stopped at Contract errors.
// Undecided is why flinch couldn't decide, or "". Full says the run mutated every covered unit; a
// scoped run still judges the obligations and tests of primitives it mutated whole, which the
// matrix marks as judged.
func Decide(problems []problem.Problem, r *matrix.Result, undecided string, full bool) Verdict {
	var v Verdict
	if undecided != "" {
		v.Reasons = append(v.Reasons, "flinch couldn't decide: "+undecided)
	}
	if n := noVerdicts(r); n > 0 {
		v.Reasons = append(v.Reasons, count(n, "mutant has", "mutants have")+" no verdict")
	}
	if len(v.Reasons) > 0 {
		v.Exit = Undecided
	}

	var failing []string
	if n := len(problems); n > 0 {
		failing = append(failing, count(n, "Contract error", "Contract errors"))
	}
	if r != nil {
		unheld := 0
		for _, m := range r.Mutants {
			if m.Status.Unheld() {
				unheld++
			}
		}
		if unheld > 0 {
			failing = append(failing, count(unheld, "unheld mutant", "unheld mutants"))
		}
		if n := len(r.Broken); n > 0 {
			failing = append(failing, count(n, "broken declaration", "broken declarations"))
		}
		hollow := 0
		for _, o := range r.Obligations {
			if o.Hollow && (full || o.Judged) {
				hollow++
			}
		}
		if hollow > 0 {
			failing = append(failing, count(hollow, "hollow obligation", "hollow obligations"))
		}
		blind := 0
		for _, t := range r.Tests {
			if t.Blind && (full || t.Judged) {
				blind++
			}
		}
		if blind > 0 {
			failing = append(failing, count(blind, "blind test", "blind tests"))
		}
	}
	v.Reasons = append(v.Reasons, failing...)
	if v.Exit == Pass && len(failing) > 0 {
		v.Exit = Fail
	}
	return v
}

// noVerdicts counts mutants without a verdict, each of which makes the run undecided (R12).
func noVerdicts(r *matrix.Result) int {
	if r == nil {
		return 0
	}
	n := 0
	for _, m := range r.Mutants {
		if m.Status == model.NoVerdict {
			n++
		}
	}
	return n
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
