// Package cli is flinch's command line. cmd/flinch is a thin wrapper around Main, so the cli
// primitive's Contract tests can run it in-process.
package cli

import "io"

// Main runs flinch with the given arguments and returns the process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return 2
}
