package ops

// These functions already return their zero values, so erasing them changes nothing.

func Zero() int { return 0 }

func Blank() (string, error) { return "", nil }

func Off() bool { return (false) }

func Rate() float64 { return 0.0 }

func Bare() (n int) { return }

func Done() { return }

// These look close but aren't zero, so they keep their erase mutants.

// Boxed returns a non-nil interface holding 0, where erase would return a nil one.
func Boxed() any { return 0 }

func One() int { return 1 }

func Pointer() *int { return new(int) }
