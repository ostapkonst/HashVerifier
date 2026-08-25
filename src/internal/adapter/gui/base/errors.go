package base

import "errors"

// ErrTabBusy is returned by Fill when a tab is currently running an operation; callers should skip the fill.
var ErrTabBusy = errors.New("tab is busy")
