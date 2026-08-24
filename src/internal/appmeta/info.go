// Package appmeta holds the application's identity (Name, Version, Link) and helpers that render the header/footer written into generated checksum files.
package appmeta

import (
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/platform/eol"
)

// Name is the application name, printed in logs and headers.
const Name = "HashVerifier"

// Link points to the project homepage; embedded in generated-file headers.
const Link = "https://github.com/ostapkonst/HashVerifier"

// Version is injected at build time via -ldflags -X github.com/ostapkonst/HashVerifier/internal/appmeta.Version.
var Version = "unknown"

// GetChecksumHeader returns the 2-line header prepended to every generated checksum file.
func GetChecksumHeader() string {
	nowUTC := time.Now().UTC()
	rfc3339 := nowUTC.Format(time.RFC3339)

	return fmt.Sprintf(
		"; Generated at %s by %s %s (%s)%s%s",
		rfc3339,
		Name,
		Version,
		Link,
		eol.PlatformEOL,
		eol.PlatformEOL,
	)
}

// FormatExportFooter returns the "Statistics" footer for a single-entry export file (status = exported).
func FormatExportFooter(entries int) string {
	return fmt.Sprintf(
		"%s; Statistics:%s"+
			";   Status: %s%s"+
			";   Entries: %d%s",
		eol.PlatformEOL,
		eol.PlatformEOL,
		StatusExported,
		eol.PlatformEOL,
		entries,
		eol.PlatformEOL,
	)
}

// FormatExportedFile composes the full content of a single-line export: header + checksum line + footer.
func FormatExportedFile(line string) string {
	return GetChecksumHeader() + line + eol.PlatformEOL + FormatExportFooter(1)
}
