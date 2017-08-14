package model

import (
	"mime/multipart"
	"fmt"
	"bytes"
	"strings"
)

const (
	headerDatumType    = "FnProject-DatumType"
	headerResultStatus = "FnProject-ResultStatus"
	headerResultCode   = "FnProject-ResultCode"
	headerHeaderPrefix = "FnProject-Header-"
	headerErrorType    = "FnProject-ErrorType"
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

	datumType := part.Header.Get(headerDatumType)
	if datumType == "" {
		return nil, fmt.Errorf("Multipart part " + part.FileName() + " cannot be read as a Datum, the " + headerDatumType + " is not present ")
	}

	switch datumType {
	case datumTypeBlob:

		blob, err := readBlob(part)
		if err != nil {
			return nil, err
		}
		return &Datum{
			Val: &Datum_Blob{Blob: blob},
		}, nil

	case datumTypeEmpty:
		return &Datum{Val: &Datum_Empty{&EmptyDatum{}}}, nil
	case datumTypeError:
		errorContentType := part.Header.Get(headerContentType)
		if errorContentType != "text/plain" {
			return nil, fmt.Errorf("Invalid error datum content type on part %s, must be text/plain", part.FileName())
		}

		errorTypeString := part.Header.Get(headerErrorType)
		if "" == errorTypeString {
			return nil, fmt.Errorf("Invalid Error Datum in part %s : no %s header defined", part.FileName(), headerErrorType)
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
			return nil, fmt.Errorf("Failed to read multipart body for %s ", part.FileName())
		}

		return &Datum{
			Val: &Datum_Error{
				&ErrorDatum{Type: pbErrorType, Message: buf.String()},
			},
		}, nil

	case datumTypeStageRef:
	case datumTypeHttpReq:
	case datumTypeHttpResp:
	default:
		return nil, fmt.Errorf("Unrecognised datum type")
	}
	return nil, fmt.Errorf("Unimplemented")
}

func readBlob(part *multipart.Part) (*BlobDatum, error) {
	contentType := part.Header.Get(headerContentType)
	if "" == contentType {
		return nil, fmt.Errorf("Mulitpart part %s is missing %s header", part.FileName(), headerContentType)
	}
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(part)
	buf.Reset()
	if err != nil {
		return nil, fmt.Errorf("Failed to read multipart body from part %s", part.FileName())
	}

	return &BlobDatum{
		ContentType: contentType,
		DataString:  buf.Bytes(),
	}, nil
}
