package appmeta

// Run-status strings written into the "Statistics:" footer of generated and exported checksum files.
const (
	StatusExported                   = "exported"
	StatusSuccess                    = "success"
	StatusCompletedWithErrors        = "completed with errors"
	StatusCompletedWithSkipped       = "completed with skipped"
	StatusCompletedWithErrorsSkipped = "completed with errors and skipped"
	StatusCanceled                   = "canceled"
	StatusFailed                     = "failed"
)
