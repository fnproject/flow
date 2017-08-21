package protocol

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
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
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
func DatumFromPart(store persistence.BlobStore, part *multipart.Part) (*model.Datum, error) {
	return readDatum(store, part, part.Header)
}

/**
 * Reads
 */
func CompletionResultFromEncapsulatedResponse(store persistence.BlobStore, r *http.Response) (*model.CompletionResult, error) {

	actualResponse, err := http.ReadResponse(bufio.NewReader(r.Body), nil)
	if err != nil {
		return nil, fmt.Errorf("Invalid encapsulated HTTP frame: %s", err.Error())
	}
	datum, err := readDatum(store, actualResponse.Body, textproto.MIMEHeader(actualResponse.Header))
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

	return &model.CompletionResult{Successful: resultStatus, Datum: datum}, nil
}

func readDatum(store persistence.BlobStore, part io.Reader, header textproto.MIMEHeader) (*model.Datum, error) {

	datumType := header.Get(headerDatumType)
	if datumType == "" {
		return nil, fmt.Errorf("Datum stream cannot be read as a Datum, the " + headerDatumType + " header is not present ")
	}

	switch datumType {
	case datumTypeBlob:

		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{
			Val: &model.Datum_Blob{Blob: blob},
		}, nil

	case datumTypeEmpty:
		return &model.Datum{Val: &model.Datum_Empty{&model.EmptyDatum{}}}, nil
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
		var pbErrorType model.ErrorDatumType
		if val, got := model.ErrorDatumType_value[pbErrorTypeString]; got {
			pbErrorType = model.ErrorDatumType(val)
		} else {
			pbErrorType = model.ErrorDatumType_unknown_error
		}

		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(part)
		if err != nil {
			return nil, fmt.Errorf("Failed to read body")
		}

		return &model.Datum{
			Val: &model.Datum_Error{
				&model.ErrorDatum{Type: pbErrorType, Message: buf.String()},
			},
		}, nil

	case datumTypeStageRef:
		stageId := header.Get(headerStageRef)
		if stageId == "" {
			return nil, fmt.Errorf("Invalid StageRef Datum")
		}
		return &model.Datum{Val: &model.Datum_StageRef{&model.StageRefDatum{StageRef: string(stageId)}}}, nil
	case datumTypeHttpReq:
		methodString := header.Get(headerMethod)
		if "" == methodString {
			return nil, fmt.Errorf("Invalid HttpReq Datum : no %s header defined", headerMethod)
		}
		method, methodRecognized := model.HttpMethod_value[strings.ToLower(methodString)]
		if !methodRecognized {
			return nil, fmt.Errorf("Invalid HttpReq Datum : http method %s is invalid", methodString)
		}
		var headers []*model.HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(headerHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &model.HttpHeader{Key: hk[len(headerHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{Val: &model.Datum_HttpReq{HttpReq: &model.HttpReqDatum{Body: blob, Headers: headers, Method: model.HttpMethod(method)}}}, nil
	case datumTypeHttpResp:
		resultCodeString := header.Get(headerResultCode)
		if "" == resultCodeString {
			return nil, fmt.Errorf("Invalid HttpResp Datum : no %s header defined", headerResultCode)
		}
		resultCode, err := strconv.ParseUint(resultCodeString, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Invalid HttpResp Datum : %s", err.Error())
		}
		var headers []*model.HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(headerHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &model.HttpHeader{Key: hk[len(headerHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{Val: &model.Datum_HttpResp{&model.HttpRespDatum{blob, headers, uint32(resultCode)}}}, nil
	default:
		return nil, fmt.Errorf("Unrecognised datum type")
	}
	return nil, fmt.Errorf("Unimplemented")
}

func readBlob(store persistence.BlobStore, part io.Reader, header textproto.MIMEHeader) (*model.BlobDatum, error) {
	contentType := header.Get(headerContentType)
	if "" == contentType {
		return nil, fmt.Errorf("Blob Datum is missing %s header", headerContentType)
	}
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(part)
	if err != nil {
		return nil, fmt.Errorf("Failed to read blob datum from body")
	}
	data := buf.Bytes()
	blob, err := store.CreateBlob(contentType, data)
	if err != nil {
		return nil, err
	}
	return blob, nil
}
