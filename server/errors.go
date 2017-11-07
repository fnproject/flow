package server

import "net/http"

// Error A server-side error with a corresponding code
type Error struct {
	HTTPStatus int
	Message    string
}

var (
	// ErrInvalidGraphID = invalid or non-existent graph ID
	ErrInvalidGraphID = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid graph id",
	}
	// ErrInvalidStageID  - invalid or non-existent stage ID
	ErrInvalidStageID = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid stage id",
	}
	// ErrInvalidDepStageID - invalid stage dependency ID
	ErrInvalidDepStageID = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid dependency stage id",
	}
	// ErrInvalidFunctionID - bad function ID
	ErrInvalidFunctionID = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid function id",
	}
	// ErrUnrecognisedCompletionOperation - invalid completer operation
	ErrUnrecognisedCompletionOperation = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid completion operation",
	}
	// ErrMissingBody Body missing from body-mandatory request
	ErrMissingBody = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Body is required",
	}
	// ErrInvalidGetTimeout - timeout is out of range
	ErrInvalidGetTimeout = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid timeoutMs parameter",
	}
	// ErrMissingOrInvalidDelay - delay param was malformed
	ErrMissingOrInvalidDelay = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid or missing delayMs parameter ",
	}
	// ErrReadingInput - I/O error reading input
	ErrReadingInput = &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "Error reading request input",
	}
	// ErrUnsupportedHTTPMethod - invalid HTTP method on request
	ErrUnsupportedHTTPMethod = &Error{
		HTTPStatus: http.StatusMethodNotAllowed,
		Message:    "Method not supported",
	}
)

func (e *Error) Error() string {
	return e.Message
}
