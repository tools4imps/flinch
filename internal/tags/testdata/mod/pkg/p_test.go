package pkg

import "testing"

// Contract: alpha/A1
func TestP(t *testing.T) {}

func TestQ(t *testing.T) {
	_ = "// Contract: alpha/A1 in a string is no tag"
}
