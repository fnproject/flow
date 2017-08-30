package model

func NewInternalErrorResult(code ErrorDatumType, message string) *CompletionResult {
	return &CompletionResult{
		Successful: false,
		Datum: &Datum{
			Val: &Datum_Error{Error: &ErrorDatum{Type: code, Message: message}},
		},
	}
}

func NewEmptyResult() *CompletionResult {
	return &CompletionResult{
		Successful: true,
		Datum:      NewEmptyDatum(),
	}
}

func NewEmptyDatum() *Datum {
	return &Datum{Val: &Datum_Empty{Empty: &EmptyDatum{}}}
}

func NewBlobDatum(blob *BlobDatum) *Datum {
	return &Datum{
		Val: &Datum_Blob{
			Blob: blob,
		},
	}
}

func NewStageRefDatum(stageID string) *Datum {
	return &Datum{
		Val: &Datum_StageRef{
			StageRef: &StageRefDatum{StageRef: stageID},
		},
	}
}

func NewSuccessfulResult(datum *Datum) *CompletionResult {
	return &CompletionResult{
		Successful: true,
		Datum:      datum,
	}
}

func NewFailedResult(datum *Datum) *CompletionResult {
	return &CompletionResult{
		Successful: false,
		Datum:      datum,
	}
}

func NewHttpReqDatum(httpreq *HttpReqDatum) *Datum {
	return &Datum{
		Val: &Datum_HttpReq{
			HttpReq: httpreq,
		},
	}
}

func NewSuccessfulStatusDatum() *Datum {
	return &Datum{
		Val: &Datum_Status{
			Status: &StatusDatum{Type: StatusDatumType_succeeded},
		},
	}
}
