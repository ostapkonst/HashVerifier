package flatpak

import (
	"os"
	"strings"
)

func IsRunningInFlatpak() bool {
	info, err := os.Stat("/.flatpak-info")
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func GetFilesystems() []string {
	data, err := os.ReadFile("/.flatpak-info")
	if err != nil {
		return nil
	}

	var filesystems []string

	inContextSection := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inContextSection = line == "[Context]"
			continue
		}

		if inContextSection && strings.HasPrefix(line, "filesystems=") {
			value := strings.TrimPrefix(line, "filesystems=")
			for _, fs := range strings.Split(value, ";") {
				fs = strings.TrimSpace(fs)
				if fs != "" {
					filesystems = append(filesystems, fs)
				}
			}

			break
		}
	}

	return filesystems
}
