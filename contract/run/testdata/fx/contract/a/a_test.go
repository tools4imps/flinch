package a_test

import (
	"os"
	"testing"
	"time"

	"example.com/fx/calc"
)

func TestAdd(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d", got)
	}
}

func TestAddMore(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Add(-1, 1); got != 0 {
		t.Errorf("Add(-1, 1) = %d", got)
	}
}

func TestAbs(t *testing.T) {
	calc.Note(t.Name())
	for name, c := range map[string][2]int{"neg": {-2, 2}, "pos": {3, 3}, "big": {-9, 9}} {
		t.Run(name, func(t *testing.T) {
			if got := calc.Abs(c[0]); got != c[1] {
				t.Errorf("Abs(%d) = %d", c[0], got)
			}
		})
	}
}

// TestSum and the two tests after it run in this order, so a hang in TestSum leaves both to restart.
func TestSum(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Sum(4); got != 6 {
		t.Errorf("Sum(4) = %d", got)
	}
}

func TestAfterSum1(t *testing.T) { calc.Note(t.Name()) }

func TestAfterSum2(t *testing.T) { calc.Note(t.Name()) }

func TestSlow(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Slow(); got != 1 {
		t.Errorf("Slow() = %d", got)
	}
}

func TestSlowToo(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Slow(); got != 1 {
		t.Errorf("Slow() = %d", got)
	}
}

// The naps give their batch a lone run time large enough to read in its budget.
func TestNap1(t *testing.T) {
	calc.Note(t.Name())
	time.Sleep(200 * time.Millisecond)
}

func TestNap2(t *testing.T) {
	calc.Note(t.Name())
	time.Sleep(200 * time.Millisecond)
}

func TestTable(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Table["one"]; got != 1 {
		t.Errorf(`Table["one"] = %d`, got)
	}
}

func TestReady(t *testing.T) {
	calc.Note(t.Name())
	if !calc.Ready() {
		t.Error("init didn't run")
	}
}

// TestReadsTestdata passes only in its own directory, the way go test runs it. The two tests after it
// restart when a crash under it names no test.
func TestReadsTestdata(t *testing.T) {
	calc.Note(t.Name())
	data, err := os.ReadFile("testdata/input.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := calc.Trim(string(data)); got != "hello" {
		t.Errorf("Trim = %q", got)
	}
}

func TestSign(t *testing.T) {
	calc.Note(t.Name())
	if calc.Sign(-5) != -1 || calc.Sign(5) != 1 || calc.Sign(0) != 0 {
		t.Error("Sign is wrong")
	}
}

func TestSignBig(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Sign(12); got != 2 {
		t.Errorf("Sign(12) = %d", got)
	}
}

// TestSignNeg reaches Sign's outer switch and never its inner one.
func TestSignNeg(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Sign(-3); got != -1 {
		t.Errorf("Sign(-3) = %d", got)
	}
}

func TestTwice(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Twice(3); got != 6 {
		t.Errorf("Twice(3) = %d", got)
	}
}

func TestBumpA(t *testing.T) {
	calc.Note(t.Name())
	calc.Bump()
}

// TestBumpB passes after TestBumpA in one process, and alone.
func TestBumpB(t *testing.T) {
	calc.Note(t.Name())
	if n := calc.Bump(); n > 2 {
		t.Errorf("Bump() = %d, want at most 2", n)
	}
}

func TestPick(t *testing.T) {
	calc.Note(t.Name())
	ch := make(chan int, 1)
	ch <- 4
	if got := calc.Pick(ch); got != 4 {
		t.Errorf("Pick = %d", got)
	}
}

func TestHold1(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Hold(); got != 7 {
		t.Errorf("Hold() = %d", got)
	}
}

func TestHold2(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Hold(); got != 7 {
		t.Errorf("Hold() = %d", got)
	}
}

func TestQuiet1(t *testing.T) { calc.Note(t.Name()) }

func TestQuiet2(t *testing.T) { calc.Note(t.Name()) }

// The chain alternates: each TestChainZ test divides and each TestChainA test adds. They run in this
// order, while every TestChainA sorts before every TestChainZ.
func TestChainA1(t *testing.T) { chainAdd(t) }

func TestChainZ1(t *testing.T) { chainDiv(t) }

func TestChainA2(t *testing.T) { chainAdd(t) }

func TestChainZ2(t *testing.T) { chainDiv(t) }

func TestChainA3(t *testing.T) { chainAdd(t) }

func TestChainZ3(t *testing.T) { chainDiv(t) }

func TestChainA4(t *testing.T) { chainAdd(t) }

func TestChainZ4(t *testing.T) { chainDiv(t) }

func TestChainA5(t *testing.T) { chainAdd(t) }

func chainAdd(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Add(1, 1); got != 2 {
		t.Errorf("Add(1, 1) = %d", got)
	}
}

func chainDiv(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Div(4, 2); got != 2 {
		t.Errorf("Div(4, 2) = %d", got)
	}
}

func TestParFast(t *testing.T) {
	t.Parallel()
	calc.Note(t.Name())
	if got := calc.Add(1, 2); got != 3 {
		t.Errorf("Add(1, 2) = %d", got)
	}
}

func TestParSum(t *testing.T) {
	t.Parallel()
	calc.Note(t.Name())
	if got := calc.Sum(3); got != 3 {
		t.Errorf("Sum(3) = %d", got)
	}
}
