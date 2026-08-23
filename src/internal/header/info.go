package header

import (
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/utils/eof"
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
		eof.PlatformEOF,
		eof.PlatformEOF,
	)
}

func FormatExportFooter(entries int) string {
	return fmt.Sprintf(
		"%s; Statistics:%s"+
			";   Status: %s%s"+
			";   Entries: %d%s",
		eof.PlatformEOF,
		eof.PlatformEOF,
		StatusExported,
		eof.PlatformEOF,
		entries,
		eof.PlatformEOF,
	)
}

// FormatExportedFile собирает содержимое файла экспорта:
// header + одна строка checksum + EOF + footer с "Entries: 1".
func FormatExportedFile(line string) string {
	return GetChecksumHeader() + line + eof.PlatformEOF + FormatExportFooter(1)
}
