package protocol

type BadProtoMessage struct {
	message string
}

func (bp *BadProtoMessage) UserMessage() string {
	return bp.Error()
}
func (bp *BadProtoMessage) Error() string {
	return bp.message
}


var (

	ErrMissingContentType =&BadProtoMessage{
		message:    "Missing " + HeaderContentType + " header ",
	}
	ErrMissingHttpMethod = &BadProtoMessage{
		message:    "Missing " + HeaderMethod + " header ",
	}
	ErrInvalidHttpMethod = &BadProtoMessage{
		message:    "Invalid " + HeaderMethod + " header ",
	}
	ErrMissingResultCode = &BadProtoMessage{
		message:    "Missing " + HeaderResultCode + " header ",
	}
	ErrInvalidResultCode = &BadProtoMessage{
		message:    "Invalid " + HeaderResultCode + " header ",
	}
	ErrMissingErrorType = &BadProtoMessage{
		message:    "Missing " + HeaderErrorType + " header ",
	}
	ErrMissingDatumType = &BadProtoMessage{
		message:    "Missing " + HeaderDatumType + " header ",
	}
	ErrInvalidDatumType = &BadProtoMessage{
		message:    "Missing " + HeaderDatumType + " header ",
	}
	ErrMissingStageRef =  &BadProtoMessage{
		message : "Missing " + HeaderStageRef + " header",
	}
	ErrInvalidContentType = &BadProtoMessage{
		message : "Unsupported content type for datum",
	}

)