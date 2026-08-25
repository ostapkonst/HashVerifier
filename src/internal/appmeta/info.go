// Package appmeta holds the application identity (Name, Version, Link) and checksum-file header/footer helpers.
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

// GetChecksumHeader builds the 2-line header so generated files self-identify their origin and timestamp.
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

// FormatExportFooter renders the Statistics block for Hash-tab exports (only Entries is reported, no Processed line).
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

// FormatExportedFile composes a one-line checksum file (entry count is hard-coded to 1).
func FormatExportedFile(line string) string {
	return GetChecksumHeader() + line + eol.PlatformEOL + FormatExportFooter(1)
}
