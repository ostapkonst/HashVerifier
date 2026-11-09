//go:build freebsd || openbsd || netbsd || dragonfly

package crash

import (
	"log/syslog"
)

// newPlatformSink wires the syslog Sink on BSDs (macOS uses Unified Logging instead).
// Connecting to /var/run/syslog may fail in sandboxed environments; the returned error
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
	msg := s.r.formatMessage(ev, true)

	for _, chunk := range chunkUTF8(msg, reportChunkSize) {
		if err := s.w.Err(chunk); err != nil {
			return err
		}
	}

	return nil
}
