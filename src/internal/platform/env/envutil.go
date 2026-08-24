package env

import (
	"os"
	"strings"
)

func Bool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on", "y", "t":
		return true
	default:
		return false
	}
}
