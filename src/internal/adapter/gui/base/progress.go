package base

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
)

// ProgressTracker wraps a GTK grid with two progress bars and a current-file label so a tab can report aggregate + per-file progress.
type ProgressTracker struct {
	gridProgress     *gtk.Grid
	totalProgress    *gtk.ProgressBar
	currFileProgress *gtk.ProgressBar
	labelCurrFileV   *gtk.Label
}

// NewProgressTracker resolves the progress grid, the two progress bars, and the current-file label from the builder by id.
func NewProgressTracker(builder *gtk.Builder, progressGridID, totalProgressID, currFileProgressID, currFileLabelID string) *ProgressTracker {
	return &ProgressTracker{
		gridProgress:     widgets.GetGrid(builder, progressGridID),
		totalProgress:    widgets.GetProgressBar(builder, totalProgressID),
		currFileProgress: widgets.GetProgressBar(builder, currFileProgressID),
		labelCurrFileV:   widgets.GetLabel(builder, currFileLabelID),
	}
}

// ActivateStopState reveals the progress grid; called when an operation starts.
func (pt *ProgressTracker) ActivateStopState() {
	pt.gridProgress.SetVisible(true)
}

// SetStartState hides the progress grid; called when an operation finishes or the tab resets.
func (pt *ProgressTracker) SetStartState() {
	pt.gridProgress.SetVisible(false)
}

// UpdateCurrentFile replaces the per-file status caption (e.g. the path currently being hashed).
func (pt *ProgressTracker) UpdateCurrentFile(status string) {
	pt.labelCurrFileV.SetText(status)
}

// UpdateTotalProgress sets the aggregate fraction (0.0–1.0) across all files in the run.
func (pt *ProgressTracker) UpdateTotalProgress(fraction float64) {
	pt.totalProgress.SetFraction(fraction)
}

// UpdateFileProgress sets the fraction (0.0–1.0) for the file currently being processed.
func (pt *ProgressTracker) UpdateFileProgress(fraction float64) {
	pt.currFileProgress.SetFraction(fraction)
}
