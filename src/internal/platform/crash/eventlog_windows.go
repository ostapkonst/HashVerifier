//go:build windows

package crash

import (
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	eventlogErrorType = 0x0001
	crashEventID      = 1000
	// windowsEventLogMax keeps ReportEventW below its per-string limit;
	// entries beyond this are truncated to retain the structured header.
	windowsEventLogMax = 31 * 1024
)

// newPlatformSink registers the generic "Application" Event Log source without
// requiring an installer or admin rights; entries appear under Source="Application"
// in Event Viewer and can be filtered by Event ID 1000.
func newPlatformSink() (Sink, error) {
	src, err := syscall.UTF16PtrFromString("Application")
	if err != nil {
		return nil, err
	}

	h, err := windows.RegisterEventSourceW(nil, src)
	if err != nil {
		return nil, err
	}

	return &eventLogSink{handle: h}, nil
}

type eventLogSink struct {
	handle windows.Handle
}

func (s *eventLogSink) Name() string { return "windows-eventlog" }

func (s *eventLogSink) Send(ev Event) error {
	msg := formatMessage(ev)
	if len(msg) > windowsEventLogMax {
		msg = msg[:windowsEventLogMax] + "\r\n...[truncated]"
	}

	p, err := syscall.UTF16PtrFromString(msg)
	if err != nil {
		return err
	}

	return windows.ReportEventW(
		s.handle,
		eventlogErrorType,
		0,
		crashEventID,
		nil,
		1,
		0,
		&p,
		nil,
	)
}
