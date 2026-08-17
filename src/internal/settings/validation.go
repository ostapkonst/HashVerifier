package settings

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

type ValidationWarning struct {
	Field   string
	Value   string
	Default string
}

func (s *Settings) Validate() []ValidationWarning {
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

	if s.Window.Width < 0 {
		warnings = append(warnings, ValidationWarning{
			Field:   "window.width",
			Value:   fmt.Sprintf("%d", s.Window.Width),
			Default: fmt.Sprintf("%d", defaults.Window.Width),
		})
		s.Window.Width = defaults.Window.Width
	}

	if s.Window.Height < 0 {
		warnings = append(warnings, ValidationWarning{
			Field:   "window.height",
			Value:   fmt.Sprintf("%d", s.Window.Height),
			Default: fmt.Sprintf("%d", defaults.Window.Height),
		})
		s.Window.Height = defaults.Window.Height
	}

	if s.Window.X < -1 {
		warnings = append(warnings, ValidationWarning{
			Field:   "window.x_pos",
			Value:   fmt.Sprintf("%d", s.Window.X),
			Default: fmt.Sprintf("%d", defaults.Window.X),
		})
		s.Window.X = defaults.Window.X
	}

	if s.Window.Y < -1 {
		warnings = append(warnings, ValidationWarning{
			Field:   "window.y_pos",
			Value:   fmt.Sprintf("%d", s.Window.Y),
			Default: fmt.Sprintf("%d", defaults.Window.Y),
		})
		s.Window.Y = defaults.Window.Y
	}

	if s.Generate.ExcludeDialog.Width < 0 {
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.exclude_dialog.width",
			Value:   fmt.Sprintf("%d", s.Generate.ExcludeDialog.Width),
			Default: fmt.Sprintf("%d", defaults.Generate.ExcludeDialog.Width),
		})
		s.Generate.ExcludeDialog.Width = defaults.Generate.ExcludeDialog.Width
	}

	if s.Generate.ExcludeDialog.Height < 0 {
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.exclude_dialog.height",
			Value:   fmt.Sprintf("%d", s.Generate.ExcludeDialog.Height),
			Default: fmt.Sprintf("%d", defaults.Generate.ExcludeDialog.Height),
		})
		s.Generate.ExcludeDialog.Height = defaults.Generate.ExcludeDialog.Height
	}

	warnings = append(warnings, resetOrder(&s.Generate.ColumnOrder, defaults.Generate.ColumnOrder, "generate.column_order")...)
	warnings = append(warnings, resetOrder(&s.Verify.ColumnOrder, defaults.Verify.ColumnOrder, "verify.column_order")...)

	if !slices.Contains(s.Generate.ColumnOrder, s.Generate.SortColumn) {
		warnings = append(warnings, ValidationWarning{
			Field:   "generate.sort_column",
			Value:   s.Generate.SortColumn,
			Default: defaults.Generate.SortColumn,
		})
		s.Generate.SortColumn = defaults.Generate.SortColumn
	}

	if !slices.Contains(s.Verify.ColumnOrder, s.Verify.SortColumn) {
		warnings = append(warnings, ValidationWarning{
			Field:   "verify.sort_column",
			Value:   s.Verify.SortColumn,
			Default: defaults.Verify.SortColumn,
		})
		s.Verify.SortColumn = defaults.Verify.SortColumn
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
	_, err := checksum.AlgorithmFromExtension(ext)
	return err == nil
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
