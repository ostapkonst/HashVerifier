//go:build darwin

package crash

/*
#include <os/log.h>
#include <stdlib.h>

static os_log_t hv_crash_log;

static void hv_oslog_init(void) {
    hv_crash_log = os_log_create("io.github.ostapkonst.HashVerifier", "crash");
    if (hv_crash_log == NULL) {
        hv_crash_log = OS_LOG_DEFAULT;
    }
}

static void hv_oslog_error(const char *msg) {
    os_log_with_type(hv_crash_log, OS_LOG_TYPE_ERROR, "%{public}s", msg);
}
*/
import "C"

import (
	"unsafe"
)

// newPlatformSink wires Unified Logging (os_log) on macOS. The subsystem matches the
// bundle identifier from Info.plist so entries are easy to filter.
func newPlatformSink(r *Reporter) (Sink, error) {
	C.hv_oslog_init()

	return &osLogSink{r: r}, nil
}

type osLogSink struct {
	r *Reporter
}

func (s *osLogSink) Name() string { return "os_log" }

func (s *osLogSink) Send(ev Event) error {
	msg := s.r.formatMessage(ev, true)

	for _, chunk := range chunkUTF8(msg, reportChunkSize) {
		c := C.CString(chunk)
		C.hv_oslog_error(c)
		C.free(unsafe.Pointer(c))
	}

	return nil
}
