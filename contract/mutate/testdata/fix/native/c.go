package native

// int two(void) { return 2; }
import "C"

func two() int {
	return int(C.two()) * 2
}
