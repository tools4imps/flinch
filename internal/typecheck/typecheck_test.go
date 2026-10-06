package typecheck

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"
	"testing"
)

const fixture = "testdata/mod"

func load(t *testing.T, tags []string, paths ...string) (*Loader, map[string]*Package) {
	t.Helper()
	// A go.work above the fixture would pull it into a workspace it isn't part of.
	t.Setenv("GOWORK", "off")
	l, pkgs, err := Load(context.Background(), fixture, tags, paths)
	if err != nil {
		t.Fatal(err)
	}
	return l, pkgs
}

func TestLoadFillsPackage(t *testing.T) {
	_, pkgs := load(t, nil, "example.com/mod/app", "example.com/mod")
	p := pkgs["example.com/mod/app"]
	if p == nil {
		t.Fatalf("no package app in %v", pkgs)
	}
	if p.Dir != "app" || p.ImportPath != "example.com/mod/app" {
		t.Errorf("Dir %q, ImportPath %q", p.Dir, p.ImportPath)
	}
	if want := []string{"app/a.go", "app/b.go"}; !reflect.DeepEqual(p.Names, want) {
		t.Errorf("Names = %v, want %v", p.Names, want)
	}
	if len(p.Files) != len(p.Names) {
		t.Fatalf("%d files for %d names", len(p.Files), len(p.Names))
	}
	for i, name := range p.Names {
		disk, err := os.ReadFile(fixture + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(p.Src[name]) != string(disk) {
			t.Errorf("Src[%q] differs from the file", name)
		}
		if got := p.Fset.Position(p.Files[i].Package).Filename; got != name {
			t.Errorf("file %d is called %q in the file set, want %q", i, got, name)
		}
	}
	if p.Types == nil || p.Types.Name() != "app" || p.Types.Scope().Lookup("Twice") == nil {
		t.Errorf("Types doesn't hold package app's Twice: %v", p.Types)
	}
	info := p.Info
	if len(info.Types) == 0 || len(info.Defs) == 0 || len(info.Uses) == 0 || len(info.Selections) == 0 {
		t.Errorf("Info is missing maps: %d types, %d defs, %d uses, %d selections",
			len(info.Types), len(info.Defs), len(info.Uses), len(info.Selections))
	}
	if root := pkgs["example.com/mod"]; root == nil || root.Dir != "." || !reflect.DeepEqual(root.Names, []string{"mod.go"}) {
		t.Errorf("the root package came back as %+v", root)
	}
}

func TestLoadHonorsTags(t *testing.T) {
	_, plain := load(t, nil, "example.com/mod/app")
	_, tagged := load(t, []string{"extra"}, "example.com/mod/app")
	if got := plain["example.com/mod/app"].Names; len(got) != 2 {
		t.Errorf("without tags the names are %v", got)
	}
	got := tagged["example.com/mod/app"].Names
	if want := []string{"app/a.go", "app/b.go", "app/tagged.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("with the extra tag the names are %v, want %v", got, want)
	}
}

func TestLoadFailsOnMissingPackage(t *testing.T) {
	t.Setenv("GOWORK", "off")
	_, _, err := Load(context.Background(), fixture, nil, []string{"example.com/mod/nothere"})
	if err == nil {
		t.Fatal("loading a package that doesn't exist succeeded")
	}
}

func TestLoadWithNoPaths(t *testing.T) {
	l, pkgs, err := Load(context.Background(), fixture, nil, nil)
	if err != nil || l == nil || len(pkgs) != 0 {
		t.Errorf("Load with no paths = %v, %v, %v", l, pkgs, err)
	}
}

func TestViable(t *testing.T) {
	l, pkgs := load(t, nil, "example.com/mod/app")
	p := pkgs["example.com/mod/app"]
	orig := string(p.Src["app/b.go"])
	replace := func(old, new string) []byte {
		t.Helper()
		if !strings.Contains(orig, old) {
			t.Fatalf("b.go doesn't contain %q", old)
		}
		return []byte(strings.Replace(orig, old, new, 1))
	}
	cases := []struct {
		name string
		src  []byte
		want bool
	}{
		{"unchanged", []byte(orig), true},
		{"a changed operator", replace("_ = http.StatusOK", "_ = http.StatusOK + 1"), true},
		{"a stranded variable", replace("return n", "return 0"), false},
		{"an unused import", replace("_ = http.StatusOK", ""), false},
		{"a type error", replace("return n", `return "n"`), false},
		{"a syntax error", replace("return n", "return n +"), false},
		{"an unknown name", replace("lib.Double(x)", "lib.Triple(x)"), false},
	}
	for _, c := range cases {
		if got := l.Viable(p, "app/b.go", c.src); got != c.want {
			t.Errorf("%s: Viable = %v, want %v", c.name, got, c.want)
		}
	}
	if l.Viable(p, "app/none.go", []byte(orig)) {
		t.Error("a file the package doesn't have was viable")
	}
}

// TestViableLeavesFileSetAlone checks that mutated files leave the file set once they're checked, so
// a long run doesn't keep a line table per mutant.
func TestViableLeavesFileSetAlone(t *testing.T) {
	l, pkgs := load(t, nil, "example.com/mod/app")
	p := pkgs["example.com/mod/app"]
	count := func() int {
		n := 0
		p.Fset.Iterate(func(*token.File) bool { n++; return true })
		return n
	}
	before := count()
	for i := 0; i < 3; i++ {
		l.Viable(p, "app/a.go", p.Src["app/a.go"])
		l.Viable(p, "app/a.go", []byte("package app\nfunc ("))
	}
	if after := count(); after != before {
		t.Errorf("the file set held %d files before and %d after", before, after)
	}
	// The package's own files and types stay as they were.
	if !l.Viable(p, "app/a.go", p.Src["app/a.go"]) {
		t.Error("the unchanged file stopped being viable")
	}
}

type fakeImporter struct{ asked []string }

func (f *fakeImporter) Import(path string) (*types.Package, error) {
	f.asked = append(f.asked, path)
	return nil, errors.New("fake")
}

func TestMappedResolvesThroughImportMap(t *testing.T) {
	f := &fakeImporter{}
	m := mapped{imp: f, m: map[string]string{"golang.org/x/net/idna": "vendor/golang.org/x/net/idna"}}
	m.Import("golang.org/x/net/idna")
	m.Import("strings")
	if want := []string{"vendor/golang.org/x/net/idna", "strings"}; !reflect.DeepEqual(f.asked, want) {
		t.Errorf("the importer was asked for %v, want %v", f.asked, want)
	}
}
