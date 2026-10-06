//go:build !linux && !darwin

package runner

// canReadRSS is false where flinch has no way to read another process's memory, so the memory
// guard watches nothing there.
const canReadRSS = false

func rss(pid int) (int64, bool) { return 0, false }
