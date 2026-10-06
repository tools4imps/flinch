package shapes_test

import (
	"testing"

	"example.com/idmod/internal/shapes"
)

// Contract: shapes/S1
func TestBoth(t *testing.T) {
	if shapes.Both(0) != 0 {
		t.Error("Both(0) != 0")
	}
}
