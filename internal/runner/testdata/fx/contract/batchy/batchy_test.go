package batchy_test

import "testing"

var dirty bool

func TestY(t *testing.T) { dirty = true }

func TestZ(t *testing.T) { dirty = false }

func TestX(t *testing.T) {
	if dirty {
		t.Error("ran after TestY with no TestZ between")
	}
}
