//go:build linux

package runner

import (
	"os"
	"strconv"
	"strings"
)

const canReadRSS = true

// rss reads a process's resident memory from /proc, in bytes, or 0 when it can't. statm's first
// field is the program's size and its second the resident set, both in pages.
func rss(pid int) int64 {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return 0
	}
	_, rest, _ := strings.Cut(string(data), " ")
	resident, _, _ := strings.Cut(rest, " ")
	pages, _ := strconv.ParseInt(resident, 10, 64)
	return pages * int64(os.Getpagesize())
}
