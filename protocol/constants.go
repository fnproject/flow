package protocol

const (
	HeaderDatumType    = "Fnproject-Datumtype"
	HeaderResultStatus = "Fnproject-Resultstatus"
	HeaderResultCode   = "Fnproject-Resultcode"
	HeaderStageRef     = "Fnproject-Stageid"
	HeaderMethod       = "Fnproject-Method"
	HeaderHeaderPrefix = "Fnproject-Header-"
	HeaderErrorType    = "Fnproject-Errortype"
	HeaderThreadId     = "Fnproject-Threadid"
	HeaderExceptional  = "FnProject-Exceptional"

	HeaderContentType = "Content-Type"

  	ResultStatusSuccess = "success"
 	ResultStatusFailure = "failure"

	DatumTypeBlob     = "blob"
	DatumTypeEmpty    = "empty"
	DatumTypeError    = "error"
	DatumTypeStageRef = "stageref"
	DatumTypeHttpReq  = "httpreq"
	DatumTypeHttpResp = "httpresp"
)
