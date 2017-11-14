package model

// NewInternalErrorResult is a shortcut to create an error result with a given message
func NewInternalErrorResult(code ErrorDatumType, message string) *CompletionResult {
	return &CompletionResult{
		Successful: false,
		Datum: &Datum{
			Val: &Datum_Error{Error: &ErrorDatum{Type: code, Message: message}},
		},
	}
}

// NewEmptyResult creates a  successful result with an empty datum attached
func NewEmptyResult() *CompletionResult {
	return &CompletionResult{
		Successful: true,
		Datum:      NewEmptyDatum(),
	}
}

// NewEmptyDatum creates a new empty datum
func NewEmptyDatum() *Datum {
	return &Datum{Val: &Datum_Empty{Empty: &EmptyDatum{}}}
}

// NewBlobDatum creates a new blob datum
func NewBlobDatum(body *BlobDatum) *Datum {
	return &Datum{
		Val: &Datum_Blob{
			Blob: body,
		},
	}
}

// NewBlobBody creates a new blob body element
func NewBlobBody(id string, length uint64, contentType string) *BlobDatum {
	return &BlobDatum{
		BlobId:      id,
		Length:      length,
		ContentType: contentType,
	}
}

// NewStageRefDatum creates a stage ref datum to a specific stage in the current graph
func NewStageRefDatum(stageID string) *Datum {
	return &Datum{
		Val: &Datum_StageRef{
			StageRef: &StageRefDatum{StageRef: stageID},
		},
	}
}

// NewSuccessfulResult creates a successful result from a given datum
func NewSuccessfulResult(datum *Datum) *CompletionResult {
	return &CompletionResult{
		Successful: true,
		Datum:      datum,
	}
}

// NewFailedResult creates a failed result from a given datum
func NewFailedResult(datum *Datum) *CompletionResult {
	return &CompletionResult{
		Successful: false,
		Datum:      datum,
	}
}

// NewHTTPReqDatum creates a datum from a HttpReq
func NewHTTPReqDatum(httpreq *HTTPReqDatum) *Datum {
	return &Datum{
		Val: &Datum_HttpReq{
			HttpReq: httpreq,
		},
	}
}

// NewStateDatum creates a graph state datum
func NewStateDatum(stateType StateDatumType) *Datum {
	return &Datum{
		Val: &Datum_State{
			State: &StateDatum{Type: stateType},
		},
	}
}
