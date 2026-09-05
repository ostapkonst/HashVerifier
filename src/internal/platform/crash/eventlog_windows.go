//go:build windows

package crash

import (
	"fmt"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	eventlogErrorType = 0x0001
	// crashEventID is the Event ID we report crashes under. 1000 matches the conventional
	// "Application Error" event ID used by WER for unhandled exceptions, so our entries
	// appear alongside generic crash reports in Event Viewer.
	crashEventID = 1000
	// windowsEventLogPerStringMax keeps each ReportEventW string below its per-string limit
	// (31,839 wide chars per Microsoft docs); entries beyond this are truncated to retain
	// the structured header fields. We emit each logical field as a separate <Data> entry, so
	// the limit is per-field rather than per-event.
	windowsEventLogPerStringMax = 31 * 1024
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
	fields := s.buildEventFields(ev)

	return windows.ReportEvent(
		s.handle,
		eventlogErrorType,
		0, // category (Task): no category assigned; entries carry no task label.
		crashEventID,
		0, // PSID: 0 = no SID; the Windows API requires uintptr (no Go nil literal).
		uint16(len(fields)),
		0,
		&fields[0],
		nil,
	)
}

// buildEventFields renders ev as a slice of UTF-16 string pointers for ReportEventW,
// where each pointer becomes its own <Data> entry in the resulting event. Windows applies
// its 31,839-char limit per string (not per event), so each field is independently
// truncated to windowsEventLogPerStringMax bytes; embedded NULs are stripped at the
// first occurrence so we never emit a partial string. Any truncation (per-length or
// at-NUL) is signalled with a trailing "..." marker.
func (s *eventLogSink) buildEventFields(ev Event) []*uint16 {
	fields := []string{
		ev.App,
		ev.Version,
		ev.Time.Format("2006-01-02T15:04:05Z07:00"),
		ev.Origin,
		fmt.Sprint(ev.PanicValue),
		string(ev.Stack),
	}

	out := make([]*uint16, 0, len(fields))

	for _, f := range fields {
		truncated := false

		if i := strings.IndexByte(f, 0); i >= 0 {
			f = f[:i]
			truncated = true
		}

		if len(f) > windowsEventLogPerStringMax {
			f = f[:windowsEventLogPerStringMax]
			truncated = true
		}

		if truncated {
			f += "..."
		}

		p, err := syscall.UTF16PtrFromString(f)
		if err != nil {
			continue
		}

		out = append(out, p)
	}

	return out
}
