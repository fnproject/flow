package protocol

const (
	// HeaderDatumType - the Datum Type for a given datum
	HeaderDatumType = "Fnproject-Datumtype"
	// HeaderResultStatus - flow result status
	HeaderResultStatus = "Fnproject-Resultstatus"
	// HeaderResultCode - HTTP datum status code
	HeaderResultCode = "Fnproject-Resultcode"
	// HeaderStageRef - flow stage reference
	HeaderStageRef = "Fnproject-Stageid"
	// HeaderCallerRef - The calling stage ID
	HeaderCallerRef = "Fnproject-Callerid"
	// HeaderMethod    - HTTP datum method
	HeaderMethod = "Fnproject-Method"
	// HeaderHeaderPrefix - HTTP header prefix
	HeaderHeaderPrefix = "Fnproject-Header-"
	// HeaderErrorType -  error datum type
	HeaderErrorType = "Fnproject-Errortype"
	// HeaderStateType - state datum type
	HeaderStateType = "Fnproject-Statetype"
	// HeaderCodeLocation - a FDK opaque code location
	HeaderCodeLocation = "Fnproject-Codeloc"
	// HeaderFlowID  - the flow ID (for return values)
	HeaderFlowID = "Fnproject-FlowId"

	// HeaderBlobID  - a blob reference
	HeaderBlobID = "Fnproject-BlobId"

	// HeaderBlobLength  - a blob reference
	HeaderBlobLength = "Fnproject-BlobLength"

	// HeaderContentType - congten type header
	HeaderContentType = "Content-Type"

	// ResultStatusSuccess - Result indication for completion results
	ResultStatusSuccess = "success"
	// ResultStatusFailure - Result indication for completion results
	ResultStatusFailure = "failure"

	// DatumTypeBlob  - datum type for Fnproject-Datumtype
	DatumTypeBlob = "blob"
	// DatumTypeEmpty  - datum type for Fnproject-Datumtype
	DatumTypeEmpty = "empty"
	// DatumTypeError  - datum type for Fnproject-Datumtype
	DatumTypeError = "error"
	// DatumTypeStageRef  - datum type for Fnproject-Datumtype
	DatumTypeStageRef = "stageref"
	// DatumTypeHTTPReq  - datum type for Fnproject-Datumtype
	DatumTypeHTTPReq = "httpreq"
	// DatumTypeHTTPResp  - datum type for Fnproject-Datumtype
	DatumTypeHTTPResp = "httpresp"
	// DatumTypeState  - datum type for Fnproject-Datumtype
	DatumTypeState = "state"
)
