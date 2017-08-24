package protocol

type badProtoMessage struct {
	message string
}

func (bp *badProtoMessage) UserMessage() string {
	return bp.Error()
}
func (bp *badProtoMessage) Error() string {
	return bp.message
}


var (

	ErrMissingContentType =&badProtoMessage{
		message:    "Missing " + HeaderContentType + " header ",
	}
	ErrMissingHttpMethod = &badProtoMessage{
		message:    "Missing " + HeaderMethod + " header ",
	}
	ErrInvalidHttpMethod = &badProtoMessage{
		message:    "Invalid " + HeaderMethod + " header ",
	}
	ErrMissingResultCode = &badProtoMessage{
		message:    "Missing " + HeaderResultCode + " header ",
	}
	ErrInvalidResultCode = &badProtoMessage{
		message:    "Invalid " + HeaderResultCode + " header ",
	}
	ErrMissingErrorType = &badProtoMessage{
		message:    "Missing " + HeaderErrorType + " header ",
	}
	ErrMissingDatumType = &badProtoMessage{
		message:    "Missing " + HeaderDatumType + " header ",
	}
	ErrInvalidDatumType = &badProtoMessage{
		message:    "Missing " + HeaderDatumType + " header ",
	}
	ErrMissingStageRef =  &badProtoMessage{
		message : "Missing " + HeaderStageRef + " header",
	}
	ErrInvalidContentType = &badProtoMessage{
		message : "Unsupported content type for datum",
	}

)