//go:build fxtag

package a_test

import (
	"testing"

	"example.com/fx/calc"
)

// TestTagged exists only when the run passes the fxtag build tag.
func TestTagged(t *testing.T) { calc.Note(t.Name()) }
