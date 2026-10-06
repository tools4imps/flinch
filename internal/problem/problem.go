// Package problem holds the Contract error, the one kind of finding every static check reports.
package problem

import (
	"fmt"
	"sort"
)

// A Problem is something wrong in the Contract's own files, such as a tag that names no obligation.
// It prints as path:line: message.
type Problem struct {
	Path    string // slash-separated, relative to the module root
	Line    int    // 1-based, or 0 when the problem concerns a whole file or directory
	Message string
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message)
	}
	return fmt.Sprintf("%s: %s", p.Path, p.Message)
}

// Sort orders problems by path, then line, then message, so every run lists them the same way.
func Sort(ps []Problem) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Message < b.Message
	})
}
