package fails_test

import "testing"

func TestFine(t *testing.T) {}

func TestFails(t *testing.T) { t.Error("always fails") }
