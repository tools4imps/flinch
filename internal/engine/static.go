// Package engine joins flinch's stages. The static half, in this file, is everything flinch contract
// does: it reads the module and the Contract and reports every Contract error, with no Go toolchain.
package engine

import (
	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/covers"
	"github.com/tools4imps/flinch/internal/declare"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
	"github.com/tools4imps/flinch/internal/tags"
)

// Options are what every run reads: where to start looking for the module, where the Contract is
// relative to the module root ("contract" when empty), and the build tags.
type Options struct {
	Dir      string
	Contract string
	Tags     []string
}

// Static is everything the static check learned, kept so a full run can build on it.
type Static struct {
	Module       source.Module
	Packages     []source.Package
	Parsed       map[string]*source.Parsed // by package dir, for packages with non-test Go files
	Contract     *contract.Contract
	Tests        []model.Test
	Ownership    *covers.Ownership
	Declarations *declare.Index
	Problems     []problem.Problem // sorted by path and line
}

// Check reads the module and its Contract and reports every Contract error at once. It returns an
// error only when flinch can't check anything at all, such as when there's no go.mod or a file
// can't be read or parsed; a problem in the Contract's own files is a Problem instead.
func Check(o Options) (*Static, error) {
	dir := o.Dir
	if dir == "" {
		dir = "."
	}
	m, err := source.FindModule(dir)
	if err != nil {
		return nil, err
	}
	pkgs, err := source.Packages(m, source.Config{Tags: o.Tags})
	if err != nil {
		return nil, err
	}
	st := &Static{Module: m, Packages: pkgs, Parsed: map[string]*source.Parsed{}}
	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}
		parsed, err := source.Parse(m, p)
		if err != nil {
			return nil, err
		}
		st.Parsed[p.Dir] = parsed
	}

	c, ps, err := contract.Load(m.Root, o.Contract)
	if err != nil {
		return nil, err
	}
	st.Contract = c
	st.Problems = append(st.Problems, ps...)

	tests, ps := tags.Scan(m, c, pkgs)
	st.Tests = tests
	st.Problems = append(st.Problems, ps...)

	own, ps := covers.Resolve(c, pkgs, st.Parsed)
	st.Ownership = own
	st.Problems = append(st.Problems, ps...)

	st.Problems = append(st.Problems, declare.Check(c, own, st.Parsed)...)
	st.Declarations = declare.NewIndex(c)
	problem.Sort(st.Problems)
	return st, nil
}
