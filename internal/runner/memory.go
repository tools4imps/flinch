package runner

import (
	"os"
	"sync"
	"time"
)

// pollEvery is how often the memory guard looks at a test process. A runaway allocation grows by a
// gigabyte or more a second, so a slower look lets it get far past the limit.
const pollEvery = 200 * time.Millisecond

// watchMemory kills p once it holds more than limit bytes, looking every pollEvery until the
// returned function is called. A process stopped this way ends without its results, so the run
// treats it like any other crash: the lone test it ran is pinned as a panic, and a batch is sent
// one test per process. A limit of zero, or a system where flinch can't read a process's memory,
// watches nothing.
func watchMemory(p *os.Process, limit int64) (stop func()) {
	if limit <= 0 || !canReadRSS {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(pollEvery)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if n := rss(p.Pid); n > limit {
					p.Kill()
					return
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}
