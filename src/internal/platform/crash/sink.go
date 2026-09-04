package crash

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Sink writes a crash Event to one destination: stderr, journald, syslog, or
// Windows Event Log. Failures are reported via the caller; Sinks must not panic
// and are expected to return quickly (the process is about to exit anyway).
type Sink interface {
	Name() string
	Send(ev Event) error
}

// stderrSink is the always-on fallback that guarantees the user sees the crash
// even when no platform OS log is reachable.
type stderrSink struct{}

func (s *stderrSink) Name() string { return "stderr" }

func (s *stderrSink) Send(ev Event) error {
	fmt.Fprintln(os.Stderr, formatMessage(ev))

	return nil
}

// formatMessage renders ev as a multi-line block readable in stderr, journald,
// syslog (truncated to ~1 KB), and Windows Event Log. The stack is the most
// useful field for diagnosis, so it is always included.
func formatMessage(ev Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "HashVerifier crashed\n")
	fmt.Fprintf(&b, "  Time:     %s\n", ev.Time.Format(time.RFC3339))
	fmt.Fprintf(&b, "  Origin:   %s\n", ev.Origin)
	fmt.Fprintf(&b, "  Panic:    %v\n", ev.PanicValue)
	fmt.Fprintf(&b, "  App:      %s %s\n", ev.App, ev.Version)
	fmt.Fprintf(&b, "  Go:       %s\n", ev.GoVersion)
	fmt.Fprintf(&b, "  OS/Arch:  %s/%s\n", ev.GOOS, ev.GOARCH)
	fmt.Fprintf(&b, "  PID:      %d\n", ev.PID)
	fmt.Fprintf(&b, "  Args:     %s\n", strings.Join(truncateArgsForDisplay(ev.Args), " "))
	fmt.Fprintf(&b, "  Stack:\n%s", string(ev.Stack))

	return b.String()
}

func truncateArgsForDisplay(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if len(a) > 256 {
			out[i] = a[:256] + "..."
		} else {
			out[i] = a
		}
	}

	return out
}
