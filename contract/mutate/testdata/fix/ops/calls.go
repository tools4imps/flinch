package ops

import (
	"fmt"
	"io"
	"log"
	"log/slog"
)

// logger embeds a *log.Logger, whose methods still belong to package log.
type logger struct{ *log.Logger }

func Calls(w io.Writer, l *log.Logger, s *slog.Logger, e logger, err error, ch chan int) {
	log.Printf("a")
	l.Println("b")
	slog.Info("c")
	s.Info("d")
	e.Printf("e")
	(log.Println)("f")
	slog.Default().Warn("g")
	fmt.Fprintln(w, "h")
	fmt.Fprintf(w,
		"%d", 1)
	err.Error()
	<-ch
	defer fmt.Fprint(w, "i")
	go fmt.Fprint(w, "j")
	close(ch)
}

// Strand's call is the only use of x, so removing it outright would strand the local.
func Strand(w io.Writer) {
	x := 1
	fmt.Fprint(w, x)
}

// Now calls function literals on the spot, and the calls belong to Now rather than to the literals.
func Now(n *int) int {
	func() { *n++ }()
	return func() int { return *n }() + 1
}
