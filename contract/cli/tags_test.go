package cli_test

import (
	"slices"
	"testing"
)

// calc's Mode has a file for builds tagged fx and one for builds that aren't, and so does its
// Contract test. --tags reaches the type-checker, which picks the file the mutant goes in, and the
// builds and test runs, which kill the mutant in the tagged file.
//
// Contract: cli/L4
func TestTagsReachEveryGoCommand(t *testing.T) {
	mod(t)
	modeFile := func(args ...string) string {
		for _, l := range listed(t, flinch(append([]string{"--dry-run"}, args...)...)) {
			if l.id == modeErase {
				return l.where
			}
		}
		return ""
	}
	if got := modeFile(); got != "calc/mode.go:6" {
		t.Errorf("with no tags, Mode's mutant is at %q, want calc/mode.go:6", got)
	}
	if got := modeFile("--tags", "fx"); got != "calc/mode_fx.go:6" {
		t.Errorf("with --tags fx, Mode's mutant is at %q, want calc/mode_fx.go:6", got)
	}
	if got := modeFile("--tags", "other, fx"); got != "calc/mode_fx.go:6" {
		t.Errorf("with --tags \"other, fx\", Mode's mutant is at %q, want calc/mode_fx.go:6", got)
	}

	r := flinch("run", "--tags", "fx", "--only", "calc", "--format", "json", "--jobs", "2")
	if r.code != 0 {
		t.Fatalf("calc holds with --tags fx:\n%s", r)
	}
	rep := parse(t, r)
	if !slices.Equal(rep.Build.Tags, []string{"fx"}) {
		t.Errorf("build tags = %q, want fx", rep.Build.Tags)
	}
	for _, m := range rep.Mutants {
		if m.ID == modeErase && (m.File != "calc/mode_fx.go" || m.Status != "killed") {
			t.Errorf("Mode's mutant is %s in %s, want killed in calc/mode_fx.go", m.Status, m.File)
		}
	}
}
