package b_test

import (
	"fmt"
	"testing"

	"example.com/fx/calc"
	"example.com/fx/words"
)

func TestOtherAdd(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Add(1, 1); got != 2 {
		t.Errorf("Add(1, 1) = %d", got)
	}
}

func TestShout(t *testing.T) {
	calc.Note(t.Name())
	if got := words.Shout("a"); got != "A" {
		t.Errorf("Shout = %q", got)
	}
}

func ExampleShout() {
	fmt.Println(words.Shout("hi"))
	// Output: HI
}

func FuzzShout(f *testing.F) {
	f.Add("a")
	f.Fuzz(func(t *testing.T, s string) {
		if words.Shout(s) == "" && s != "" {
			t.Error("Shout lost its input")
		}
	})
}
