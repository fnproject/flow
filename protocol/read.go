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


// DatumFromPart reads a model Datum Object from a multipart part
func DatumFromPart(store persistence.BlobStore, part *multipart.Part) (*model.Datum, error) {
	return readDatum(store, part, part.Header)
}

// DatumFromPart reads a model Datum Object from a multipart part
func DatumFromRequest(store persistence.BlobStore, req *http.Request) (*model.Datum, error) {
	return readDatum(store, req.Body, textproto.MIMEHeader(req.Header))
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
	statusString := actualResponse.Header.Get(HeaderResultStatus)

	var resultStatus bool
	switch statusString {
	case "success":
		resultStatus = true
	case "failure":
		resultStatus = false
	default:
		return nil, fmt.Errorf("Invalid result status header %s: \"%s\" ", HeaderResultStatus, statusString)
	}

	return &model.CompletionResult{Successful: resultStatus, Datum: datum}, nil
}

func readDatum(store persistence.BlobStore, part io.Reader, header textproto.MIMEHeader) (*model.Datum, error) {

	datumType := header.Get(HeaderDatumType)
	if datumType == "" {
		return nil,ErrMissingDatumType
	}

	switch datumType {
	case DatumTypeBlob:

		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{
			Val: &model.Datum_Blob{Blob: blob},
		}, nil

	case DatumTypeEmpty:
		return &model.Datum{Val: &model.Datum_Empty{&model.EmptyDatum{}}}, nil
	case DatumTypeError:
		errorContentType := header.Get(HeaderContentType)
		if errorContentType == ""{
			return nil,ErrMissingContentType
		}
		if errorContentType != "text/plain" {
			return nil, ErrInvalidContentType
		}

		errorTypeString := header.Get(HeaderErrorType)
		if "" == errorTypeString {
			return nil, ErrMissingErrorType
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

	case DatumTypeStageRef:
		stageId := header.Get(HeaderStageRef)
		if stageId == "" {
			return nil, ErrMissingStageRef
		}
		return &model.Datum{Val: &model.Datum_StageRef{&model.StageRefDatum{StageRef: string(stageId)}}}, nil
	case DatumTypeHttpReq:
		methodString := header.Get(HeaderMethod)
		if "" == methodString {
			return nil, ErrMissingHttpMethod
		}
		method, methodRecognized := model.HttpMethod_value[strings.ToLower(methodString)]
		if !methodRecognized {
			return nil, ErrInvalidHttpMethod
		}
		var headers []*model.HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(HeaderHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &model.HttpHeader{Key: hk[len(HeaderHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{Val: &model.Datum_HttpReq{HttpReq: &model.HttpReqDatum{Body: blob, Headers: headers, Method: model.HttpMethod(method)}}}, nil
	case DatumTypeHttpResp:
		resultCodeString := header.Get(HeaderResultCode)
		if "" == resultCodeString {
			return nil, ErrMissingResultCode
		}
		resultCode, err := strconv.ParseUint(resultCodeString, 10, 32)
		if err != nil {
			return nil, ErrInvalidResultCode
		}
		var headers []*model.HttpHeader
		for hk, hvs := range header {
			if strings.HasPrefix(strings.ToLower(hk), strings.ToLower(HeaderHeaderPrefix)) {
				for _, hv := range hvs {
					headers = append(headers, &model.HttpHeader{Key: hk[len(HeaderHeaderPrefix):], Value: hv})
				}
			}
		}
		blob, err := readBlob(store, part, header)
		if err != nil {
			return nil, err
		}
		return &model.Datum{Val: &model.Datum_HttpResp{&model.HttpRespDatum{blob, headers, uint32(resultCode)}}}, nil
	default:
		return nil, ErrInvalidDatumType
	}
}

func readBlob(store persistence.BlobStore, part io.Reader, header textproto.MIMEHeader) (*model.BlobDatum, error) {
	contentType := header.Get(HeaderContentType)
	if "" == contentType {
		return nil, ErrMissingContentType
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
