package runner

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Test binaries run in their package's directory, the way go test runs them, so a mutant that makes
// code write files by relative path writes them into the source tree. One stray .go file there and
// the package no longer builds, which would leave every later mutant without a verdict. So flinch
// notes what each suite's directory holds before any test runs, and whenever the last of a suite's
// running test processes ends, it removes whatever has appeared since. Waiting for the last one
// keeps it from deleting a file a test that is still running depends on.

// listTree lists every file and directory under dir, by path relative to dir.
func listTree(dir string) (map[string]bool, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		seen[rel] = true
		return nil
	})
	return seen, err
}

// enter records that a test process is about to run in the suite's directory.
func (s *suite) enter() {
	s.mu.Lock()
	s.active++
	s.mu.Unlock()
}

// leave records that a test process has ended. New .go files directly in the suite's directory go
// at once, since they break the next build and no test writes one on purpose. When this was the last
// process running there, everything else that wasn't there before the run goes too.
func (s *suite) leave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active--
	s.sweepGo()
	if s.active > 0 || s.before == nil {
		return
	}
	filepath.WalkDir(s.abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.abs, p)
		if err != nil || s.before[rel] {
			return nil
		}
		os.RemoveAll(p)
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
}

// sweepGo removes .go files directly in the suite's directory that weren't there before the run. The
// caller holds s.mu.
func (s *suite) sweepGo() {
	if s.before == nil {
		return
	}
	entries, err := os.ReadDir(s.abs)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".go" && !s.before[e.Name()] {
			os.Remove(filepath.Join(s.abs, e.Name()))
		}
	}
}

// sweepAllGo clears stray .go files from every suite's directory.
func (b *Baseline) sweepAllGo() {
	for _, s := range b.suites {
		s.mu.Lock()
		s.sweepGo()
		s.mu.Unlock()
	}
}
