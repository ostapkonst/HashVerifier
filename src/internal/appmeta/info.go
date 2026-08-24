package appmeta

import (
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/platform/eol"
)

const (
	Name = "HashVerifier"
	Link = "https://github.com/ostapkonst/HashVerifier"
)

// Version устанавливается при компиляции через -ldflags -X.
var Version = "unknown"

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

// FormatExportedFile собирает содержимое файла экспорта:
// header + одна строка checksum + EOF + footer с "Entries: 1".
func FormatExportedFile(line string) string {
	return GetChecksumHeader() + line + eol.PlatformEOL + FormatExportFooter(1)
}
