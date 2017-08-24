package server

import "net/http"

type ServerErr struct {
	HttpStatus int
	Message    string
}

var (
	ErrInvalidGraphId = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid graph id",
	}
	ErrInvalidStageId = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid stage id",
	}
	ErrInvalidDepStageId = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid dependency stage id",
	}
	ErrInvalidFunctionId = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid function id",
	}
	ErrUnrecognisedCompletionOperation = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid completion operation",
	}
	ErrMissingBody = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Body is required",
	}

	ErrMissingOrInvalidDelay = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Invalid or missing delayMs parameter ",
	}

	ErrReadingInput = &ServerErr{
		HttpStatus: http.StatusBadRequest,
		Message:    "Error reading request input",
	}
	ErrUnsupportedHttpMethod = &ServerErr{
		HttpStatus: http.StatusMethodNotAllowed,
		Message:    "Method not supported",
	}
)

func (e *ServerErr) Error() string {
	return e.Message
}
