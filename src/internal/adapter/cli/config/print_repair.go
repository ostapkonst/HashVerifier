package config

import (
	"fmt"
	"strings"

	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

// printRepairs renders a structured "Repairs applied" block to stdout for
// user-facing config-* commands. Empty slice skips printing anything.
// Prints a leading blank line to separate from preceding output.
func printRepairs(warnings []settings.ValidationWarning) {
	if len(warnings) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Repairs applied (%d):\n", len(warnings))
	fmt.Println(strings.Repeat("=", 80))

	for _, w := range warnings {
		fmt.Printf("  %s: %q -> %q\n", w.Field, w.Value, w.Default)
	}
}
