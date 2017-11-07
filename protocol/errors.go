package protocol

// BadProtoMessage wraps a bad protocol message
type BadProtoMessage struct {
	message string
}

// UserMessage is the user-facing (external) string to publish for a given message
func (bp *BadProtoMessage) UserMessage() string {
	return bp.Error()
}
func (bp *BadProtoMessage) Error() string {
	return bp.message
}

var (
	// ErrMissingContentType  - no content type in datum
	ErrMissingContentType = &BadProtoMessage{
		message: "Missing " + HeaderContentType + " header ",
	}
	// ErrMissingHTTPMethod - no http method in invoke request
	ErrMissingHTTPMethod = &BadProtoMessage{
		message: "Missing " + HeaderMethod + " header ",
	}
	// ErrInvalidHTTPMethod  - unknown HTTP method in invoke request
	ErrInvalidHTTPMethod = &BadProtoMessage{
		message: "Invalid " + HeaderMethod + " header ",
	}
	// ErrMissingResultCode -  HttpDatum is missing result code
	ErrMissingResultCode = &BadProtoMessage{
		message: "Missing " + HeaderResultCode + " header ",
	}
	// ErrInvalidResultCode - HttpDatum has invalid result code
	ErrInvalidResultCode = &BadProtoMessage{
		message: "Invalid " + HeaderResultCode + " header ",
	}
	// ErrMissingErrorType - error datum is missing error type
	ErrMissingErrorType = &BadProtoMessage{
		message: "Missing " + HeaderErrorType + " header ",
	}
	// ErrMissingDatumType - no datum type on datum message
	ErrMissingDatumType = &BadProtoMessage{
		message: "Missing " + HeaderDatumType + " header ",
	}
	// ErrInvalidDatumType - unknown datum type on datum message
	ErrInvalidDatumType = &BadProtoMessage{
		message: "Missing " + HeaderDatumType + " header ",
	}
	// ErrMissingStageRef - no stage ref on stage-ref datum message
	ErrMissingStageRef = &BadProtoMessage{
		message: "Missing " + HeaderStageRef + " header",
	}
	// ErrInvalidContentType - error datums may only have text/plain content
	ErrInvalidContentType = &BadProtoMessage{
		message: "Unsupported content type for datum",
	}
)
