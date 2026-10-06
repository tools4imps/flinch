//go:build darwin

package runner

import (
	"os/exec"
	"strconv"
	"strings"
)

const canReadRSS = true

// rss asks ps for a process's resident memory, in bytes, or 0 when it can't. The standard library
// has no other way to read another process's memory on macOS without cgo.
func rss(pid int) int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb * 1024
}
