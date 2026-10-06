package order_test

import "testing"

var setUp bool

func TestSetup(t *testing.T) { setUp = true }

func TestDepends(t *testing.T) {
	if !setUp {
		t.Error("needs TestSetup to run first")
	}
}
