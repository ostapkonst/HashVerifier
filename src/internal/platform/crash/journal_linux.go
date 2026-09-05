//go:build linux

package crash

import (
	"github.com/coreos/go-systemd/v22/journal"
)

// newPlatformSink wires the journald Sink. journal.Send connects on first use
// and returns an error if /run/systemd/journal/socket is missing (e.g. inside
// a chroot or a Docker container without journal mounted); that error is treated
// as "sink unavailable" and the Reporter falls back to stderr-only.
func newPlatformSink(r *Reporter) (Sink, error) {
	return &journalSink{r: r}, nil
}

type journalSink struct {
	r *Reporter
}

func (s *journalSink) Name() string { return "systemd-journal" }

func (s *journalSink) Send(ev Event) error {
	msg := s.r.formatMessage(ev, true)
	vars := map[string]string{
		// Native-protocol entries skip journald's credential-based SYSLOG_IDENTIFIER fill;
		// set it so journalctl -t <App> behaves the way the docs promise.
		"SYSLOG_IDENTIFIER": ev.App,
		// journalctl renders DOCUMENTATION as a hyperlink in its output, per
		// systemd.journal-fields(7): the canonical way to attach a "report here" URL.
		"DOCUMENTATION":          s.r.link + "/issues",
		"HASHVERIFIER_VERSION":   ev.Version,
		"HASHVERIFIER_ORIGIN":    ev.Origin,
		"HASHVERIFIER_GOVERSION": ev.GoVersion,
	}

	return journal.Send(msg, journal.PriErr, vars)
}
