package runner

import (
	"reflect"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
)

func TestReadProfile(t *testing.T) {
	data := []byte(`mode: set
example.com/m/calc/calc.go:10.24,10.38 1 1
example.com/m/calc/calc.go:12.18,13.12 1 0
example.com/m/main.go:3.13,5.2 2 1
example.org/dep/dep.go:1.1,2.2 1 1
`)
	dirs := map[string]string{"example.com/m/calc": "calc", "example.com/m": "."}
	got, err := readProfile(data, dirs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]span{
		"calc/calc.go": {{10, 24, 10, 38}},
		"main.go":      {{3, 13, 5, 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readProfile = %v, want %v", got, want)
	}
}

func TestReadProfileRejectsGarbage(t *testing.T) {
	if _, err := readProfile([]byte("mode: set\nnot a profile line\n"), nil); err == nil {
		t.Error("want an error for an unreadable line")
	}
}

func TestSpans(t *testing.T) {
	s := span{10, 5, 12, 2}
	for _, c := range []struct {
		line, col int
		in        bool
	}{{10, 5, true}, {10, 4, false}, {11, 1, true}, {12, 2, true}, {12, 3, false}, {9, 80, false}} {
		if got := s.contains(c.line, c.col); got != c.in {
			t.Errorf("contains(%d, %d) = %v", c.line, c.col, got)
		}
	}
	if !s.overlaps(span{12, 2, 14, 1}) || s.overlaps(span{12, 3, 14, 1}) || !s.overlaps(span{1, 1, 99, 1}) {
		t.Error("overlaps is wrong at the edges")
	}
}

func TestCoverageBlocks(t *testing.T) {
	a := model.TestRef{Primitive: "p", Name: "TestA"}
	b := model.TestRef{Primitive: "p", Name: "TestB"}
	c := coverage{}
	c.add(b, map[string][]span{"f.go": {{3, 1, 4, 1}, {1, 1, 2, 1}}})
	c.add(a, map[string][]span{"f.go": {{3, 1, 4, 1}}})
	got := c.blocks()["f.go"]
	want := []block{{span{1, 1, 2, 1}, []model.TestRef{b}}, {span{3, 1, 4, 1}, []model.TestRef{a, b}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocks = %v, want %v", got, want)
	}
}
