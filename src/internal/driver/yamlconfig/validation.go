package yamlconfig

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
)

// ValidationWarning records a single field that Validate changed: Value is the rejected
// value and Default is what the field was set to — except `hash.algorithms`, where only the
// unrecognized entries are dropped, so Default names the full default list while the field
// keeps the recognized entries.
type ValidationWarning struct {
	Field   string
	Value   string
	Default string
}

// Validate enforces s against allowed enums and column rules, mutating invalid fields back to defaults.
// Mutating instead of erroring lets Load recover when settings.yaml contains unknown values.
// Unknown keys are ignored by the parser, and a value of the wrong YAML type is not repaired —
// the parse fails first, so `config show`/`config edit` report the file as corrupt (exit 78).
func (s *SettingsBase) Validate() []ValidationWarning {
	var warnings []ValidationWarning

	defaults := DefaultSettings()

	switch s.Window.RestoreMode {
	case RestoreModeDefault, RestoreModeSize, RestoreModePosition, RestoreModeAll:
	default:
		warnings = append(warnings, ValidationWarning{
			Field:   "window.restore_mode",
			Value:   string(s.Window.RestoreMode),
			Default: string(defaults.Window.RestoreMode),
		})
		s.Window.RestoreMode = defaults.Window.RestoreMode
	}

	switch s.Window.WindowState {
	case WindowStateNormal, WindowStateMaximized, WindowStateFullscreen:
	default:
		warnings = append(warnings, ValidationWarning{
			Field:   "window.window_state",
			Value:   string(s.Window.WindowState),
			Default: string(defaults.Window.WindowState),
		})
		s.Window.WindowState = defaults.Window.WindowState
	}

	switch s.Generate.SortOrder {
	case SortOrderAsc, SortOrderDesc:
	default:
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.sort_order",
			Value:   string(s.Generate.SortOrder),
			Default: string(defaults.Generate.SortOrder),
		})
		s.Generate.SortOrder = defaults.Generate.SortOrder
	}

	switch s.Verify.SortOrder {
	case SortOrderAsc, SortOrderDesc:
	default:
		warnings = append(warnings, ValidationWarning{
			Field:   "verify.sort_order",
			Value:   string(s.Verify.SortOrder),
			Default: string(defaults.Verify.SortOrder),
		})
		s.Verify.SortOrder = defaults.Verify.SortOrder
	}

	switch s.Discover.SortOrder {
	case SortOrderAsc, SortOrderDesc:
	default:
		warnings = append(warnings, ValidationWarning{
			Field:   "discover.sort_order",
			Value:   string(s.Discover.SortOrder),
			Default: string(defaults.Discover.SortOrder),
		})
		s.Discover.SortOrder = defaults.Discover.SortOrder
	}

	if !s.Discover.SourcesDialog.FilesToScan.Valid() {
		warnings = append(warnings, ValidationWarning{
			Field:   "discover.sources_dialog.files_to_scan",
			Value:   string(s.Discover.SourcesDialog.FilesToScan),
			Default: string(defaults.Discover.SourcesDialog.FilesToScan),
		})
		s.Discover.SourcesDialog.FilesToScan = defaults.Discover.SourcesDialog.FilesToScan
	}

	if !s.Discover.SourcesDialog.Action.Valid() {
		warnings = append(warnings, ValidationWarning{
			Field:   "discover.sources_dialog.action",
			Value:   string(s.Discover.SourcesDialog.Action),
			Default: string(defaults.Discover.SourcesDialog.Action),
		})
		s.Discover.SourcesDialog.Action = defaults.Discover.SourcesDialog.Action
	}

	if !isKnownAlgorithm(s.Generate.Algorithm) {
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.algorithm",
			Value:   s.Generate.Algorithm,
			Default: defaults.Generate.Algorithm,
		})
		s.Generate.Algorithm = defaults.Generate.Algorithm
	}

	if len(s.Hash.Algorithms) > 0 {
		valid := make([]string, 0, len(s.Hash.Algorithms))

		var invalidValues []string

		for _, a := range s.Hash.Algorithms {
			if isKnownAlgorithm(a) {
				valid = append(valid, a)
			} else {
				invalidValues = append(invalidValues, a)
			}
		}

		if len(invalidValues) > 0 {
			warnings = append(warnings, ValidationWarning{
				Field:   "hash.algorithms",
				Value:   strings.Join(invalidValues, ","),
				Default: strings.Join(defaults.Hash.Algorithms, ","),
			})
			if len(valid) == 0 {
				valid = defaults.Hash.Algorithms
			}

			s.Hash.Algorithms = valid
		}
	}

	warnings = append(warnings, resetSize(&s.Window.Width, defaults.Window.Width, "window.width")...)
	warnings = append(warnings, resetSize(&s.Window.Height, defaults.Window.Height, "window.height")...)
	warnings = append(warnings, resetSize(&s.Generate.ExcludeDialog.Width, defaults.Generate.ExcludeDialog.Width, "generate.exclude_dialog.width")...)
	warnings = append(warnings, resetSize(&s.Generate.ExcludeDialog.Height, defaults.Generate.ExcludeDialog.Height, "generate.exclude_dialog.height")...)
	warnings = append(warnings, resetSize(&s.Discover.SourcesDialog.Width, defaults.Discover.SourcesDialog.Width, "discover.sources_dialog.width")...)
	warnings = append(warnings, resetSize(&s.Discover.SourcesDialog.Height, defaults.Discover.SourcesDialog.Height, "discover.sources_dialog.height")...)

	warnings = append(warnings, resetOrder(&s.Generate.ColumnOrder, defaults.Generate.ColumnOrder, "generate.column_order")...)
	warnings = append(warnings, resetOrder(&s.Verify.ColumnOrder, defaults.Verify.ColumnOrder, "verify.column_order")...)
	warnings = append(warnings, resetOrder(&s.Discover.ColumnOrder, defaults.Discover.ColumnOrder, "discover.column_order")...)

	if !slices.Contains(s.Generate.ColumnOrder, s.Generate.SortColumn) ||
		!slices.Contains(s.GenerateSortableColumns(), s.Generate.SortColumn) {
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.sort_column",
			Value:   s.Generate.SortColumn,
			Default: defaults.Generate.SortColumn,
		})
		s.Generate.SortColumn = defaults.Generate.SortColumn
	}

	if !slices.Contains(s.Verify.ColumnOrder, s.Verify.SortColumn) ||
		!slices.Contains(s.VerifySortableColumns(), s.Verify.SortColumn) {
		warnings = append(warnings, ValidationWarning{
			Field:   "verify.sort_column",
			Value:   s.Verify.SortColumn,
			Default: defaults.Verify.SortColumn,
		})
		s.Verify.SortColumn = defaults.Verify.SortColumn
	}

	if !slices.Contains(s.Discover.ColumnOrder, s.Discover.SortColumn) ||
		!slices.Contains(s.DiscoverSortableColumns(), s.Discover.SortColumn) {
		warnings = append(warnings, ValidationWarning{
			Field:   "discover.sort_column",
			Value:   s.Discover.SortColumn,
			Default: defaults.Discover.SortColumn,
		})
		s.Discover.SortColumn = defaults.Discover.SortColumn
	}

	warnings = append(warnings, resetOrder(&s.Window.TabOrder, defaults.Window.TabOrder, "window.tab_order")...)

	maxPage := len(s.Window.TabOrder) - 1
	if s.Window.CurrentPage < 0 || s.Window.CurrentPage > maxPage {
		warnings = append(warnings, ValidationWarning{
			Field:   "window.current_page",
			Value:   fmt.Sprintf("%d", s.Window.CurrentPage),
			Default: fmt.Sprintf("%d", defaults.Window.CurrentPage),
		})
		s.Window.CurrentPage = defaults.Window.CurrentPage
	}

	return warnings
}

func isKnownAlgorithm(ext string) bool {
	return algorithm.IsCanonicalAlgorithm(ext)
}

// resetSize resets a negative persisted size dimension to its default, reporting a warning.
func resetSize(field *int, defaultValue int, fieldName string) []ValidationWarning {
	if *field >= 0 {
		return nil
	}

	warning := ValidationWarning{
		Field:   fieldName,
		Value:   fmt.Sprintf("%d", *field),
		Default: fmt.Sprintf("%d", defaultValue),
	}

	*field = defaultValue

	return []ValidationWarning{warning}
}

func resetOrder(out *[]string, validItems []string, fieldName string) []ValidationWarning {
	validSet := make(map[string]bool, len(validItems))
	for _, v := range validItems {
		validSet[v] = true
	}

	counts := make(map[string]int, len(*out))
	for _, item := range *out {
		counts[item]++
	}

	var missing []string

	for _, v := range validItems {
		if counts[v] == 0 {
			missing = append(missing, v)
		}
	}

	var unknown, duplicates []string

	for item, count := range counts {
		switch {
		case !validSet[item]:
			unknown = append(unknown, item)
		case count > 1:
			duplicates = append(duplicates, item)
		}
	}

	sort.Strings(unknown)
	sort.Strings(duplicates)

	if len(missing) == 0 && len(unknown) == 0 && len(duplicates) == 0 {
		return nil
	}

	*out = append([]string{}, validItems...)

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ","))
	}

	if len(unknown) > 0 {
		parts = append(parts, "unknown: "+strings.Join(unknown, ","))
	}

	if len(duplicates) > 0 {
		parts = append(parts, "duplicates: "+strings.Join(duplicates, ","))
	}

	return []ValidationWarning{{
		Field:   fieldName,
		Value:   strings.Join(parts, "; "),
		Default: strings.Join(validItems, ","),
	}}
}
