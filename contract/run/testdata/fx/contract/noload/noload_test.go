package noload_test

import (
	"testing"

	"example.com/fx/missing"
)

func TestNothing(t *testing.T) { missing.Do() }
