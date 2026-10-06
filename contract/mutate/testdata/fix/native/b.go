package native

// int one(void) { return 1; }
import "C"

func one() int {
	return int(C.one()) + 1
}
