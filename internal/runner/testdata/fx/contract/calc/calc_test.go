package calc_test

import (
	"os"
	"testing"

	"example.com/fx/calc"
)

func TestAdd(t *testing.T) {
	if got := calc.Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d", got)
	}
}

func TestAbs(t *testing.T) {
	t.Run("neg", func(t *testing.T) {
		if got := calc.Abs(-2); got != 2 {
			t.Errorf("Abs(-2) = %d", got)
		}
	})
	t.Run("pos", func(t *testing.T) {
		if got := calc.Abs(3); got != 3 {
			t.Errorf("Abs(3) = %d", got)
		}
	})
}

func TestDiv(t *testing.T) {
	if got := calc.Div(6, 3); got != 2 {
		t.Errorf("Div(6, 3) = %d", got)
	}
}

func TestSum(t *testing.T) {
	if got := calc.Sum(4); got != 6 {
		t.Errorf("Sum(4) = %d", got)
	}
}

func TestSlow(t *testing.T) {
	if got := calc.Slow(); got != 1 {
		t.Errorf("Slow() = %d", got)
	}
}

func TestTable(t *testing.T) {
	if got := calc.Table["one"]; got != 1 {
		t.Errorf(`Table["one"] = %d`, got)
	}
}

func TestReady(t *testing.T) {
	if !calc.Ready() {
		t.Error("init didn't run")
	}
}

// TestReadsTestdata passes only when it runs in its own directory, the way go test runs it.
func TestReadsTestdata(t *testing.T) {
	data, err := os.ReadFile("testdata/input.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := calc.Trim(string(data)); got != "hello" {
		t.Errorf("Trim = %q", got)
	}
}

func TestLater(t *testing.T) {}
