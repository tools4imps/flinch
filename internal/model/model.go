// Package model holds the plain data that passes between flinch's stages: mutants, Contract tests,
// and the rows a run records. It has no behavior of its own.
package model

// A Mutant is one small change to covered code.
type Mutant struct {
	ID         string // the readable id, as package mutantid prints it
	Hash       string // the first 12 hex digits of the id's SHA-256
	Primitive  string // the primitive whose covers block owns the unit
	Dir        string // the package directory, slash-separated, relative to the module root
	ImportPath string
	File       string // slash-separated, relative to the module root
	Line, Col  int    // where the change starts, 1-based
	Start, End int    // the byte range replaced in the original file
	Text       string // the bytes that replace [Start, End)
	Operator   string
	Unit       string // the innermost unit holding the change
	Top        string // the top-level unit holding the change
	Original   string // the replaced source, whitespace collapsed, as in the id
	Replace    string // the replacement, whitespace collapsed, as in the id
	// Erase marks a function's erase mutant, which runs before the function's other mutants.
	Erase bool
	// Linked marks a change in a package-level variable's initializer or an init function. Coverage
	// puts no counters there, so every Contract test whose binary links the package reaches it.
	Linked bool
}

// Apply returns src with the mutant's change made.
func (m Mutant) Apply(src []byte) []byte {
	out := make([]byte, 0, len(src)-(m.End-m.Start)+len(m.Text))
	out = append(out, src[:m.Start]...)
	out = append(out, m.Text...)
	return append(out, src[m.End:]...)
}

// A TestRef names one Contract test.
type TestRef struct {
	Primitive string
	Name      string // the top-level function, such as TestSkipsVendor
}

func (t TestRef) String() string { return t.Primitive + "/" + t.Name }

// A Test is a Contract test and the obligations it names.
type Test struct {
	TestRef
	Kind        string   // "Test", "Example" or "Fuzz"
	Dir         string   // the primitive's directory, slash-separated, relative to the module root
	ImportPath  string   // the test package's import path
	File        string   // slash-separated, relative to the module root
	Line        int      // the line of the func keyword
	Obligations []string // full ids, such as "skip/K1"
}

// KillKind says how a test failed against a mutant.
type KillKind string

const (
	Assertion KillKind = "assertion" // the test reported a failure and returned
	Panic     KillKind = "panic"     // the test, or the process before any test started, panicked
	Timeout   KillKind = "timeout"   // the test was still running when the time budget ran out
)

// A Kill is one test failing against one mutant.
type Kill struct {
	Test     TestRef
	Subtests []string // the failing subtests, by their full names, when any failed
	Kind     KillKind
}

// A Row is what running one mutant recorded.
type Row struct {
	Ran      []TestRef // every covering test that ran to a pass or a fail
	Kills    []Kill
	Complete bool   // every covering test has a pass or a fail of its own
	Verdict  string // empty when the row has a verdict; otherwise why it has none
}

// Status is where a mutant ended up.
type Status string

const (
	Killed    Status = "killed"
	Lived     Status = "lived"     // Contract tests ran it and every one passed
	Unreached Status = "unreached" // no Contract test runs it
	Erased    Status = "erased"    // a function's erase mutant that lived
	Skipped   Status = "skipped"   // a finer mutant in an erased function; never run
	Declared  Status = "declared"  // unheld, and a declaration in mutants.md accounts for it
	NoVerdict Status = "no verdict"
)

// Unheld reports whether a status fails the build unless a declaration accounts for it.
func (s Status) Unheld() bool { return s == Lived || s == Unreached || s == Erased }
