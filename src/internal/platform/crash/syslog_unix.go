//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package crash

import (
	"log/syslog"
)

// newPlatformSink wires the syslog Sink on BSDs and macOS. Connecting to
// /var/run/syslog may fail in sandboxed environments; the returned error
// causes the Reporter to fall back to stderr-only.
func newPlatformSink(r *Reporter) (Sink, error) {
	w, err := syslog.New(syslog.LOG_ERR|syslog.LOG_USER, "HashVerifier")
	if err != nil {
		return nil, err
	}

	return &syslogSink{w: w, r: r}, nil
}

type syslogSink struct {
	w *syslog.Writer
	r *Reporter
}

func (s *syslogSink) Name() string { return "syslog" }

func (s *syslogSink) Send(ev Event) error {
	// Traditional syslog truncates messages to ~1 KB; the stack line in
	// formatMessage is preserved up to that limit, which is enough for triage.
	return s.w.Err(s.r.formatMessage(ev, true))
}
