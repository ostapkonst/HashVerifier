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

// ApplyRenames renames each file on disk; an existing target is refused so no file is overwritten.
// Every action is attempted and its outcome reported so a single failure does not abort the batch.
func ApplyRenames(actions []embedded.RenameAction) []RenameOutcome {
	outcomes := make([]RenameOutcome, 0, len(actions))

	for _, action := range actions {
		outcomes = append(outcomes, RenameOutcome{Action: action, Err: applyRename(action)})
	}

	return outcomes
}

func applyRename(action embedded.RenameAction) error {
	if _, err := os.Lstat(action.To); err == nil {
		return fmt.Errorf("target already exists: %s", action.To)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Rename(action.From, action.To)
}
