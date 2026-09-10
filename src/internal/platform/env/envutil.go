// Package env reads boolean environment variables with a tolerant truthy-value parser.
package env

import (
	"os"
	"strings"
)

// Bool returns true when key is one of 1, true, yes, on, y, t (case-insensitive); anything else, including empty, means false.
func Bool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on", "y", "t":
		return true
	default:
		return false
	}
}

// NoConfig reports whether HASHVERIFIER_NO_CONFIG is set in the environment —
// the single place that reads the env var.
func NoConfig() bool {
	return Bool("HASHVERIFIER_NO_CONFIG")
}
