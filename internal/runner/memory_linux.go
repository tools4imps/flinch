//go:build linux

package runner

import (
	"os"
	"strconv"
	"strings"
)

const canReadRSS = true

// rss reads a process's resident memory from /proc, in bytes. The second field of statm is the
// resident set in pages.
func rss(pid int) (int64, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, false
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return pages * int64(os.Getpagesize()), true
}
