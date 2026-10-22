package discover

import (
	"errors"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
)

// RenameOutcome is the result of applying one RenameAction.
type RenameOutcome struct {
	Action embedded.RenameAction
	Err    error
}

// applyRename renames one file; an existing target is refused so no file is overwritten.
func applyRename(action embedded.RenameAction) error {
	if _, err := os.Lstat(action.To); err == nil {
		return fmt.Errorf("target already exists: %s", action.To)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Rename(action.From, action.To)
}
