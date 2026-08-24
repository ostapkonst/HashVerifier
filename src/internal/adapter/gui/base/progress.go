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

func (pt *ProgressTracker) UpdateCurrentFile(status string) {
	pt.labelCurrFileV.SetText(status)
}

func (pt *ProgressTracker) UpdateTotalProgress(fraction float64) {
	pt.totalProgress.SetFraction(fraction)
}

func (pt *ProgressTracker) UpdateFileProgress(fraction float64) {
	pt.currFileProgress.SetFraction(fraction)
}
