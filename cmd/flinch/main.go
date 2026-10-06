// Command flinch holds Go code to its Contract by mutation.
package main

import (
	"os"

	"github.com/tools4imps/flinch/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
