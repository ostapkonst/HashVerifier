package crash

import (
	"fmt"
	"os"
	"strconv"
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
type stderrSink struct {
	r *Reporter
}

func (s *stderrSink) Name() string { return "stderr" }

func (s *stderrSink) Send(ev Event) error {
	fmt.Fprintln(os.Stderr, s.r.formatMessage(ev, false))

	return nil
}

// formatMessage renders ev as a multi-line block readable in stderr, journald,
// syslog (truncated to ~1 KB), and Windows Event Log. includeStack=false replaces
// the goroutine dump with a short footer pointing the reader to the OS log and
// the issue tracker — that is the right shape for user-facing stderr where a raw
// stack is noise; OS-log sinks pass true to keep the full stack.
func (r *Reporter) formatMessage(ev Event, includeStack bool) string {
	var b strings.Builder
	b.WriteString("HashVerifier crashed\n\n")

	panicStr := fmt.Sprint(ev.PanicValue)

	b.WriteString("  Time:     " + ev.Time.Format(time.RFC3339) + "\n")
	b.WriteString("  Origin:   " + ev.Origin + "\n")

	if panicStr == "" {
		b.WriteString("  Panic:    <empty>\n")
	} else {
		lines := strings.Split(panicStr, "\n")
		b.WriteString("  Panic:    " + lines[0] + "\n")

		for _, ln := range lines[1:] {
			// Skip blank lines from the panic message: the indented continuation
			// already visually separates blocks (e.g. gotk3's error text from its
			// closure stack), so a trailing empty line would just add noise.
			if ln == "" {
				continue
			}

			b.WriteString("            " + ln + "\n")
		}
	}

	b.WriteString("  App:      " + ev.App + " " + ev.Version + "\n")
	b.WriteString("  Go:       " + ev.GoVersion + "\n")
	b.WriteString("  OS/Arch:  " + ev.GOOS + "/" + ev.GOARCH + "\n")
	b.WriteString("  PID:      " + strconv.Itoa(ev.PID) + "\n")
	b.WriteString("  Args:     " + strings.Join(truncateArgsForDisplay(ev.Args), " ") + "\n")

	if includeStack {
		b.WriteString("\nStack:\n\n")
		b.Write(ev.Stack)
	} else {
		b.WriteString("\nDetails have been written to the system log.\n")
		b.WriteString("Please report this issue at: " + r.link + "/issues\n")
	}

	return b.String()
}

func truncateArgsForDisplay(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if len(a) > 256 {
			a = a[:256]
		}

		// Quoted form: a stray newline from bash $'…' or any other control byte
		// becomes visible (\n, \t, \xNN) and the Args block stays single-line.
		out[i] = strconv.Quote(a)
	}

	return out
}
