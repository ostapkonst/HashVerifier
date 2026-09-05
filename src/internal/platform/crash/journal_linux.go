//go:build linux

package crash

import (
	"fmt"

	"github.com/coreos/go-systemd/v22/journal"
)

// newPlatformSink wires the journald Sink. journal.Send connects on first use
// and returns an error if /run/systemd/journal/socket is missing (e.g. inside
// a chroot or a Docker container without journal mounted); that error is treated
// as "sink unavailable" and the Reporter falls back to stderr-only.
func newPlatformSink() (Sink, error) {
	return &journalSink{}, nil
}

type journalSink struct{}

func (s *journalSink) Name() string { return "systemd-journal" }

func (s *journalSink) Send(ev Event) error {
	msg := formatMessage(ev)
	vars := map[string]string{
		// Native-protocol entries skip journald's credential-based SYSLOG_IDENTIFIER fill;
		// set it so journalctl -t <App> behaves the way the docs promise.
		"SYSLOG_IDENTIFIER":      ev.App,
		"HASHVERIFIER_APP":       ev.App,
		"HASHVERIFIER_VERSION":   ev.Version,
		"HASHVERIFIER_ORIGIN":    ev.Origin,
		"HASHVERIFIER_PANIC":     fmt.Sprint(ev.PanicValue),
		"HASHVERIFIER_GOVERSION": ev.GoVersion,
	}

	return journal.Send(msg, journal.PriErr, vars)
}
