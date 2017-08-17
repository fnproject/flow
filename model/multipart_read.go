package model

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"bufio"
)

const (
	headerDatumType    = "Fnproject-Datumtype"
	headerResultStatus = "Fnproject-Resultstatus"
	headerResultCode   = "Fnproject-Resultcode"
	headerStageRef     = "Fnproject-Stageid"
	headerMethod       = "Fnproject-Method"
	headerHeaderPrefix = "Fnproject-Header-"
	headerErrorType    = "Fnproject-Errortype"
	headerContentType  = "Content-Type"

	datumTypeBlob     = "blob"
	datumTypeEmpty    = "empty"
	datumTypeError    = "error"
	datumTypeStageRef = "stageref"
	datumTypeHttpReq  = "httpreq"
	datumTypeHttpResp = "httpresp"
)

// DatumFromPart reads a model Datum Object from a multipart part
func DatumFromPart(part *multipart.Part) (*Datum, error) {
	return readDatum(part, part.Header)
}

// DatumFromHttpRequest reads a model Datum Object from an HTTP request
func DatumFromHttpRequest(r *http.Request) (*Datum, error) {
	return readDatum(r.Body, textproto.MIMEHeader(r.Header))
}

/**
 * Reads
 */
func CompletionResultFromEncapsulatedResponse(r *http.Response) (*CompletionResult, error) {

	actualResponse, err := http.ReadResponse(bufio.NewReader(r.Body), nil)
	if err != nil {
		return nil, fmt.Errorf("Invalid encapsulated HTTP frame: %s", err.Error())
	}
	datum,err:=  readDatum(actualResponse.Body, textproto.MIMEHeader(actualResponse.Header))
	if err != nil {
		return nil, err
	}
	statusString := actualResponse.Header.Get(headerResultStatus)

	var resultStatus bool
	switch statusString {
	case "success":
		resultStatus = true
	case "failure":
		resultStatus = false
	default:
		return nil, fmt.Errorf("Invalid result status header %s: \"%s\" ", headerResultStatus, statusString)
	}

	return &CompletionResult{Successful: resultStatus, Datum: datum}, nil
}

// DatumFromHttpResponse reads a model Datum Object from an HTTP response
func DatumFromHttpResponse(r *http.Response) (*Datum, error) {
	return readDatum(r.Body, textproto.MIMEHeader(r.Header))
}


func readDatum(part io.Reader, header textproto.MIMEHeader) (*Datum, error) {

	datumType := header.Get(headerDatumType)
	if datumType == "" {
		return nil, fmt.Errorf("Datum stream cannot be read as a Datum, the " + headerDatumType + " header is not present ")
	}

	switch datumType {
	case datumTypeBlob:

		blob, err := readBlob(part, header)
		if err != nil {
			return nil, err
		}
		return &Datum{
			Val: &Datum_Blob{Blob: blob},
		}, nil

	case datumTypeEmpty:
		return &Datum{Val: &Datum_Empty{&EmptyDatum{}}}, nil
	case datumTypeError:
		errorContentType := header.Get(headerContentType)
		if errorContentType != "text/plain" {
			return nil, fmt.Errorf("Invalid error datum content type, must be text/plain")
		}

		errorTypeString := header.Get(headerErrorType)
		if "" == errorTypeString {
			return nil, fmt.Errorf("Invalid Error Datum : no %s header defined", headerErrorType)
		}

		pbErrorTypeString := strings.Replace(errorTypeString, "-", "_", -1)

		// Unrecognised errors are coerced to unknown
		var pbErrorType ErrorDatumType
		if val, got := ErrorDatumType_value[pbErrorTypeString]; got {
			pbErrorType = ErrorDatumType(val)
		} else {
			pbErrorType = ErrorDatumType_unknown_error
		}

		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(part)
		if err != nil {
			return nil, fmt.Errorf("Failed to read body")
		}

		return &Datum{
			Val: &Datum_Error{
				&ErrorDatum{Type: pbErrorType, Message: buf.String()},
			},
		}, nil

	case datumTypeStageRef:
		stageId := header.Get(headerStageRef)
		if stageId == "" {
			return nil, fmt.Errorf("Invalid StageRef Datum")
		}
		return &Datum{Val: &Datum_StageRef{&StageRefDatum{StageRef: string(stageId)}}}, nil
	case datumTypeHttpReq:
		methodString := header.Get(headerMethod)
		if "" == methodString {
			return nil, fmt.Errorf("Invalid HttpReq Datum : no %s header defined", headerMethod)
		}
		method, methodRecognized := HttpMethod_value[strings.ToLower(methodString)]
		if !methodRecognized {
			return nil, fmt.Errorf("Invalid HttpReq Datum : http method %s is invalid", methodString)
		}
		var headers []*HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(headerHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &HttpHeader{Key: hk[len(headerHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(part, header)
		if err != nil {
			return nil, err
		}
		return &Datum{Val: &Datum_HttpReq{&HttpReqDatum{blob, headers, HttpMethod(method)}}}, nil
	case datumTypeHttpResp:
		resultCodeString := header.Get(headerResultCode)
		if "" == resultCodeString {
			return nil, fmt.Errorf("Invalid HttpResp Datum : no %s header defined", headerResultCode)
		}
		resultCode, err := strconv.ParseUint(resultCodeString, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Invalid HttpResp Datum : %s", err.Error())
		}
		var headers []*HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(headerHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &HttpHeader{Key: hk[len(headerHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(part, header)
		if err != nil {
			return nil, err
		}
		return &Datum{Val: &Datum_HttpResp{&HttpRespDatum{blob, headers, uint32(resultCode)}}}, nil
	default:
		return nil, fmt.Errorf("Unrecognised datum type")
	}
	return nil, fmt.Errorf("Unimplemented")
}

func readBlob(part io.Reader, header textproto.MIMEHeader) (*BlobDatum, error) {
	contentType := header.Get(headerContentType)
	if "" == contentType {
		return nil, fmt.Errorf("Blob Datum is missing %s header", headerContentType)
	}
	buf := new(bytes.Buffer)
	buf.Reset()
	_, err := buf.ReadFrom(part)
	if err != nil {
		return nil, fmt.Errorf("Failed to read blob datum from body")
	}

	return &BlobDatum{
		ContentType: contentType,
		DataString:  buf.Bytes(),
	}, nil
}
