package crash

import (
	"os"
	"runtime"
	"time"
)

const (
	stackLimit = 64 * 1024
	argLimit   = 1024
)

// Event is one recovered panic: stack, message, goroutine name and recovery time.
// Sinks consume Event; reporters never mutate it after construction.
type Event struct {
	Time       time.Time
	App        string
	Version    string
	GoVersion  string
	GOOS       string
	GOARCH     string
	PID        int
	Args       []string
	PanicValue any
	Stack      []byte
	Origin     string
}

func buildEvent(r *Reporter, panicValue any, origin string) Event {
	buf := make([]byte, stackLimit)
	n := runtime.Stack(buf, false)

	return Event{
		Time:       time.Now().UTC(),
		App:        r.app,
		Version:    r.version,
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		PID:        os.Getpid(),
		Args:       truncateArgs(os.Args),
		PanicValue: panicValue,
		Stack:      buf[:n],
		Origin:     origin,
	}
}

func truncateArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if len(a) > argLimit {
			out[i] = a[:argLimit] + "...<truncated>"
		} else {
			out[i] = a
		}
	}

	return out
}
