package report

import (
	"encoding/json"
	"io"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
)

// The JSON shapes. Field order here is the order in the output, every list is sorted before it's
// written, and the one map, timing, is written by encoding/json in key order. Lists are never null,
// so a consumer can range over any of them without a check.
type jsonReport struct {
	SchemaVersion      string             `json:"schema_version"`
	Summary            jsonSummary        `json:"summary"`
	Build              jsonBuild          `json:"build"`
	ContractErrors     []jsonProblem      `json:"contract_errors"`
	BrokenDeclarations []jsonProblem      `json:"broken_declarations"`
	Mutants            []jsonMutant       `json:"mutants"`
	Obligations        []jsonObligation   `json:"obligations"`
	Tests              []jsonTest         `json:"tests"`
	Outside            []string           `json:"outside"`
	Undecided          string             `json:"undecided"`
	Exit               int                `json:"exit"`
	Timing             map[string]float64 `json:"timing"`
}

type jsonSummary struct {
	Scoped    bool           `json:"scoped"`
	Since     string         `json:"since"`
	Mutants   int            `json:"mutants"`
	Killed    int            `json:"killed"`
	Lived     int            `json:"lived"`
	Unreached int            `json:"unreached"`
	Erased    int            `json:"erased"`
	Skipped   int            `json:"skipped"`
	Declared  jsonDeclared   `json:"declared"`
	NoVerdict int            `json:"no_verdict"`
	Unheld    int            `json:"unheld"`
	Unviable  int            `json:"unviable"`
	Score     *float64       `json:"score"` // killed / (killed + unheld) as a percentage; null when both are zero
	Obls      jsonOblSummary `json:"obligations"`
	Tests     jsonTestCounts `json:"tests"`
}

type jsonDeclared struct {
	Total      int `json:"total"`
	Equivalent int `json:"equivalent"`
	Unpromised int `json:"unpromised"`
	Wildcard   int `json:"wildcard"`
}

type jsonOblSummary struct {
	Total        int `json:"total"`
	Holding      int `json:"holding"`
	Hollow       int `json:"hollow"`
	CrashOnly    int `json:"crash_only"`
	WithoutSole  int `json:"without_sole_holds"`
	NotJudged    int `json:"empty_not_judged"`
	SoleHoldsSum int `json:"sole_holds_total"`
}

type jsonTestCounts struct {
	Total int `json:"total"`
	Blind int `json:"blind"`
}

type jsonBuild struct {
	Go     string   `json:"go"`
	GOOS   string   `json:"goos"`
	GOARCH string   `json:"goarch"`
	Tags   []string `json:"tags"`
	Flinch string   `json:"flinch"`
}

type jsonProblem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type jsonMutant struct {
	ID          string           `json:"id"`
	Hash        string           `json:"hash"`
	Primitive   string           `json:"primitive"`
	File        string           `json:"file"`
	Line        int              `json:"line"`
	Col         int              `json:"col"`
	Unit        string           `json:"unit"`
	Operator    string           `json:"operator"`
	Original    string           `json:"original"`
	Replacement string           `json:"replacement"`
	Status      model.Status     `json:"status"`
	KilledBy    []jsonKill       `json:"killed_by"`
	RanBy       []string         `json:"ran_by"`
	Obligations []string         `json:"obligations"`
	Declaration *jsonDeclaration `json:"declaration"`
	OutsideOnly bool             `json:"outside_only"`
	Verdict     string           `json:"verdict,omitempty"` // why it has no verdict
}

type jsonKill struct {
	Test        string         `json:"test"`
	Subtests    []string       `json:"subtests"`
	Obligations []string       `json:"obligations"`
	Kind        model.KillKind `json:"kind"`
}

type jsonDeclaration struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Reason   string `json:"reason"`
	Wildcard bool   `json:"wildcard"`
}

type jsonObligation struct {
	ID        string   `json:"id"`
	Tests     []string `json:"tests"`
	Holds     []string `json:"holds"`
	SoleHolds []string `json:"sole_holds"`
	Judged    bool     `json:"judged"`
	Hollow    bool     `json:"hollow"`
	CrashOnly bool     `json:"crash_only"`
}

type jsonTest struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Obligations []string `json:"obligations"`
	Kills       []string `json:"kills"`
	Ran         int      `json:"ran"`
	Judged      bool     `json:"judged"`
	Blind       bool     `json:"blind"`
}

// JSON writes the whole report for agents and scripts. The same report gives the same bytes, apart
// from the timing object.
func JSON(w io.Writer, r *Report) error {
	c := count(r.Matrix)
	out := jsonReport{
		SchemaVersion: SchemaVersion,
		Summary: jsonSummary{
			Scoped: r.Scoped || r.Since != "", Since: r.Since,
			Mutants: c.Mutants, Killed: c.Killed, Lived: c.Lived, Unreached: c.Unreached, Erased: c.Erased,
			Skipped: c.Skipped, NoVerdict: c.NoVerdict, Unheld: c.Unheld, Unviable: r.Unviable,
			Declared: jsonDeclared{Total: c.Declared, Equivalent: c.Equivalent, Unpromised: c.Unpromised, Wildcard: c.Wildcard},
			Obls: jsonOblSummary{Total: c.Obligations, Holding: c.Holding, Hollow: c.Hollow, CrashOnly: c.CrashOnly,
				WithoutSole: c.NoSole, NotJudged: c.Unjudged},
			Tests: jsonTestCounts{Total: c.Tests, Blind: c.Blind},
		},
		Build:              jsonBuild{Go: r.GoVersion, GOOS: r.GOOS, GOARCH: r.GOARCH, Tags: sortedStrs(r.Tags), Flinch: r.Version},
		ContractErrors:     problems(r.Problems),
		BrokenDeclarations: []jsonProblem{},
		Mutants:            []jsonMutant{},
		Obligations:        []jsonObligation{},
		Tests:              []jsonTest{},
		Outside:            sortedStrs(r.Outside),
		Undecided:          r.Undecided,
		Exit:               r.Exit,
		Timing:             map[string]float64{},
	}
	if s, ok := c.score(); ok {
		out.Summary.Score = &s
	}
	for name, d := range r.Timing {
		out.Timing[name] = float64(d.Milliseconds()) / 1000
	}
	if m := r.Matrix; m != nil {
		out.BrokenDeclarations = problems(m.Broken)
		for _, mu := range sortedMutants(m.Mutants) {
			out.Mutants = append(out.Mutants, mutant(mu))
		}
		for _, o := range sortedObligations(m.Obligations) {
			out.Summary.Obls.SoleHoldsSum += len(o.SoleHolds)
			out.Obligations = append(out.Obligations, jsonObligation{
				ID: o.ID, Tests: refs(o.Tests), Holds: sortedStrs(o.Holds), SoleHolds: sortedStrs(o.SoleHolds),
				Judged: o.Judged, Hollow: o.Hollow, CrashOnly: o.CrashOnly,
			})
		}
		for _, t := range m.Tests {
			out.Tests = append(out.Tests, jsonTest{
				Name: t.TestRef.String(), Kind: t.Kind, File: t.File, Line: t.Line,
				Obligations: sortedStrs(t.Obligations), Kills: sortedStrs(t.Kills), Ran: t.Ran,
				Judged: t.Judged, Blind: t.Blind,
			})
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Mutant ids hold "->" and "<", which people read more easily unescaped.
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

func mutant(m matrix.Mutant) jsonMutant {
	j := jsonMutant{
		ID: m.ID, Hash: m.Hash, Primitive: m.Primitive, File: m.File, Line: m.Line, Col: m.Col,
		Unit: m.Unit, Operator: m.Operator, Original: m.Original, Replacement: m.Replace,
		Status: m.Status, KilledBy: []jsonKill{}, RanBy: refs(m.RanBy), Obligations: sortedStrs(m.Obligations),
		OutsideOnly: m.OutsideOnly, Verdict: m.Verdict,
	}
	for _, k := range m.Kills {
		j.KilledBy = append(j.KilledBy, jsonKill{
			Test: k.Test.String(), Subtests: sortedStrs(k.Subtests), Obligations: sortedStrs(k.Obligations), Kind: k.Kind,
		})
	}
	if d := m.Declaration; d != nil {
		j.Declaration = &jsonDeclaration{Kind: d.Kind, Path: d.Path, Line: d.Line, Reason: d.Reason, Wildcard: d.Wildcard}
	}
	return j
}

func problems(ps []problem.Problem) []jsonProblem {
	sorted := append([]problem.Problem(nil), ps...)
	problem.Sort(sorted)
	out := []jsonProblem{}
	for _, p := range sorted {
		out = append(out, jsonProblem{Path: p.Path, Line: p.Line, Message: p.Message})
	}
	return out
}

// refs names tests as primitive/Name, sorted.
func refs(ts []model.TestRef) []string {
	out := []string{}
	for _, t := range ts {
		out = append(out, t.String())
	}
	return sortedStrs(out)
}
