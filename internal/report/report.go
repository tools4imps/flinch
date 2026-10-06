// Package report prints a run's findings, as text for people and as JSON for agents. The text leads
// with what has to change, and every finding says what would clear it. The JSON lists everything and
// comes out byte for byte the same for the same run, apart from its timing object.
package report

import (
	"time"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/problem"
)

// SchemaVersion is the JSON schema's version. Additive changes bump the minor version, and consumers
// ignore keys they don't know.
const SchemaVersion = "1.0"

// A Report is everything a run has to say.
type Report struct {
	Version   string // flinch's version
	GoVersion string // the toolchain the run used, such as "go1.26.1"
	GOOS      string
	GOARCH    string
	Tags      []string
	Contract  string // the Contract directory, relative to the module root; "" means "contract"
	Scoped    bool   // the run mutated less than every covered unit
	Since     string // the --since ref, when there was one
	Problems  []problem.Problem
	Matrix    *matrix.Result // nil when the run stopped at Contract errors
	Outside   []string       // module packages no covers block names
	Undecided string         // why flinch couldn't decide, or ""
	Exit      int
	Timing    map[string]time.Duration // wall time per phase; the only part that varies between identical runs
	Unviable  int                      // mutants dropped because they didn't type-check
}

// contractDir is the Contract directory the report's advice points at.
func (r *Report) contractDir() string {
	if r.Contract == "" {
		return "contract"
	}
	return r.Contract
}

// mutantsFile is where a primitive's declarations go.
func (r *Report) mutantsFile(primitive string) string {
	return r.contractDir() + "/" + primitive + "/mutants.md"
}
