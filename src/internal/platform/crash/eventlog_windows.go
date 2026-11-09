//go:build windows

package crash

import (
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	eventlogErrorType = 0x0001
	// crashEventID matches the conventional "Application Error" ID used by WER, so our entries
	// appear alongside generic crash reports in Event Viewer.
	crashEventID = 1000
	// windowsEventLogMax keeps ReportEventW below its 31,839-wide-char per-string limit
	// (Microsoft docs); over-length messages are truncated to retain the header. The
	// truncation may overshoot by up to 3 bytes to keep the cut on a rune boundary.
	windowsEventLogMax = 31 * 1024
)

// newPlatformSink registers the generic "Application" Event Log source without
// requiring an installer or admin rights; entries appear under Source="Application"
// in Event Viewer and can be filtered by Event ID 1000.
func newPlatformSink(r *Reporter) (Sink, error) {
	src, err := syscall.UTF16PtrFromString("Application")
	if err != nil {
		return nil, err
	}

	h, err := windows.RegisterEventSource(nil, src)
	if err != nil {
		return nil, err
	}

	return &eventLogSink{handle: h, r: r}, nil
}

type eventLogSink struct {
	handle windows.Handle
	r      *Reporter
}

func (s *eventLogSink) Name() string { return "windows-eventlog" }

func (s *eventLogSink) Send(ev Event) error {
	msg := s.r.formatMessage(ev, true)
	if len(msg) > windowsEventLogMax {
		msg = truncateUTF8(msg, windowsEventLogMax) + "\r\n...[truncated]"
	}

	p, err := syscall.UTF16PtrFromString(msg)
	if err != nil {
		return err
	}

	return windows.ReportEvent(
		s.handle,
		eventlogErrorType,
		0,
		crashEventID,
		0, // PSID: 0 = no SID; the Windows API requires uintptr (no Go nil literal).
		1,
		0,
		&p,
		nil,
	)
}

// truncateUTF8 cuts s without splitting a UTF-8 sequence; a cut inside a rune would
// otherwise reach the Event Log as U+FFFD. The result may exceed max by up to 3 bytes —
// the length of the longest UTF-8 rune — which the Event Log limit leaves room for.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:utf8Cut(s, max)]
}
