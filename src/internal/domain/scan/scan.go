// Package scan defines the file-selection and action policy for the discover flow.
package scan

import (
	"fmt"
	"strings"
)

// FilesToScan selects which files a discovery run processes.
type FilesToScan string

// FilesToScan values: all files, only names that embed a checksum, or only names without one.
const (
	FilesToScanAll             FilesToScan = "all"
	FilesToScanWithChecksum    FilesToScan = "with_checksum"
	FilesToScanWithoutChecksum FilesToScan = "without_checksum"
)

// DefaultFilesToScan preserves the historical behavior of verifying only checksum-bearing names.
const DefaultFilesToScan = FilesToScanWithChecksum

// Valid reports whether f is one of the defined FilesToScan values.
func (f FilesToScan) Valid() bool {
	switch f {
	case FilesToScanAll, FilesToScanWithChecksum, FilesToScanWithoutChecksum:
		return true
	default:
		return false
	}
}

// ParseFilesToScan parses a scope value, accepting case-insensitive text with '_' or '-' as the
// word separator; an empty or unknown value is an error.
func ParseFilesToScan(raw string) (FilesToScan, error) {
	switch normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), "-", "_"); normalized {
	case string(FilesToScanAll):
		return FilesToScanAll, nil
	case string(FilesToScanWithChecksum):
		return FilesToScanWithChecksum, nil
	case string(FilesToScanWithoutChecksum):
		return FilesToScanWithoutChecksum, nil
	default:
		return "", fmt.Errorf("invalid files to scan %q (want all, with_checksum, or without_checksum)", raw)
	}
}

// Action selects what a discovery run does with the files in scope.
type Action string

// Action values: report only, add missing checksums, add or fix checksums, or remove checksums.
const (
	ActionReportOnly Action = "report_only"
	ActionAddMissing Action = "add_missing"
	ActionAddOrFix   Action = "add_or_fix"
	ActionRemove     Action = "remove"
)

// DefaultAction is the non-mutating action selected on a fresh settings file.
const DefaultAction = ActionReportOnly

// Valid reports whether a is one of the defined Action values.
func (a Action) Valid() bool {
	switch a {
	case ActionReportOnly, ActionAddMissing, ActionAddOrFix, ActionRemove:
		return true
	default:
		return false
	}
}

// ParseAction parses an action value, accepting case-insensitive text with '_' or '-' as the
// word separator; an empty or unknown value is an error.
func ParseAction(raw string) (Action, error) {
	switch normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), "-", "_"); normalized {
	case string(ActionReportOnly):
		return ActionReportOnly, nil
	case string(ActionAddMissing):
		return ActionAddMissing, nil
	case string(ActionAddOrFix):
		return ActionAddOrFix, nil
	case string(ActionRemove):
		return ActionRemove, nil
	default:
		return "", fmt.Errorf("invalid action %q (want report_only, add_missing, add_or_fix, or remove)", raw)
	}
}
