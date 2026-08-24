// Package env reads boolean environment variables with a tolerant truthy-value parser.
package env

import (
	"os"
	"strings"
)

// Bool returns true when the value of key is one of: 1, true, yes, on, y, t (case-insensitive). Empty or unknown values mean false.
func Bool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on", "y", "t":
		return true
	default:
		return false
	}
}
