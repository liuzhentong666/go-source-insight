// Package fault defines public, path-free errors shared by services and HTTP.
package fault

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrInvalidTarget      = errors.New("target must be a valid path within the project's configured source root")
	ErrExcludedTarget     = errors.New("target is excluded from project source operations")
	ErrUnsafePath         = errors.New("source path contains a link or escapes its project")
	ErrSourceUnavailable  = errors.New("project source could not be read completely")
	ErrInvalidSource      = errors.New("source analysis failed; check Go syntax")
	ErrQueueFull          = errors.New("job queue is full; try again later")
	ErrStorage            = errors.New("job storage is unavailable")
	ErrConflict           = errors.New("job state changed")
	ErrProjectBusy        = errors.New("project indexing or question answering is busy; try again later")
	ErrInvalidQuestion    = errors.New("question must contain between 1 and 4096 characters")
	ErrAIUnavailable      = errors.New("Ollama or vector storage is unavailable")
	ErrIndexFailed        = errors.New("source indexing failed")
	ErrAskFailed          = errors.New("question answering failed")
	ErrTimeout            = errors.New("operation timed out")
	ErrCanceled           = errors.New("operation canceled")
	ErrStopping           = errors.New("service is shutting down")
	ErrPublicationUnknown = errors.New("index publication could not be confirmed; check the published index before retry")
)

// Public never includes wrapped filesystem, SQL, or parser error details.
func Public(err error) (code, message string) {
	for _, item := range []struct {
		err  error
		code string
	}{
		{ErrNotFound, "not_found"}, {ErrInvalidTarget, "invalid_target"},
		{ErrExcludedTarget, "excluded_target"},
		{ErrUnsafePath, "unsafe_path"}, {ErrSourceUnavailable, "source_unavailable"},
		{ErrInvalidSource, "invalid_source"}, {ErrQueueFull, "queue_full"},
		{ErrStorage, "storage_unavailable"}, {ErrConflict, "state_conflict"},
		{ErrProjectBusy, "project_busy"}, {ErrInvalidQuestion, "invalid_question"},
		{ErrAIUnavailable, "ai_unavailable"}, {ErrIndexFailed, "index_failed"}, {ErrAskFailed, "ask_failed"},
		{ErrTimeout, "timeout"}, {ErrCanceled, "canceled"},
		{ErrStopping, "service_stopping"},
		{ErrPublicationUnknown, "index_publication_unknown"},
	} {
		if errors.Is(err, item.err) {
			return item.code, item.err.Error()
		}
	}
	return "operation_failed", "operation failed"
}
