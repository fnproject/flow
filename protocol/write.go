package protocol

import (
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/fnproject/flow/model"
	"net/http"
)

// WriteTarget is an abstraction for writing protocol frames to a given output format (e.g. http, multipart body)
type WriteTarget interface {
	Write(header textproto.MIMEHeader, body []byte) error
}

type partTarget struct {
	writer *multipart.Writer
}

// Write implements WriteTarget
func (t *partTarget) Write(header textproto.MIMEHeader, body []byte) error {
	w, err := t.writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return nil
}

type httpTarget struct {
	writer http.ResponseWriter
}

// NewMultipartTarget creates a write target based on a multipart string
func NewMultipartTarget(writer *multipart.Writer) WriteTarget {
	return &partTarget{
		writer: writer,
	}
}

//NewResponseWriterTarget creates a write target from an http.ResponseWriter
func NewResponseWriterTarget(writer http.ResponseWriter) WriteTarget {
	return &httpTarget{
		writer: writer,
	}
}

// Write implements WriteTarget
func (t *httpTarget) Write(header textproto.MIMEHeader, body []byte) error {
	for k, vs := range header {
		for _, v := range vs {
			t.writer.Header().Add(k, v)
		}
	}
	_, err := t.writer.Write([]byte(body))
	if err != nil {
		return err
	}
	return nil
}

// WriteDatum emits a datum to the specified target
func WriteDatum(target WriteTarget, datum *model.Datum) error {
	return writeDatumToTarget(target, textproto.MIMEHeader{}, datum)
}

func writeDatumToTarget(target WriteTarget, header textproto.MIMEHeader, datum *model.Datum) error {

	switch datum.Val.(type) {

	case *model.Datum_Blob:
		blob := datum.GetBlob()
		header.Add(HeaderDatumType, DatumTypeBlob)
		addBlobHeaders(header, blob)
		return target.Write(header, []byte{})

	case *model.Datum_Empty:
		header.Add(HeaderDatumType, DatumTypeEmpty)
		return target.Write(header, []byte{})

	case *model.Datum_Error:
		header.Add(HeaderDatumType, DatumTypeError)
		errorType := model.ErrorDatumType_name[int32(datum.GetError().Type)]
		stringTypeName := strings.Replace(errorType, "_", "-", -1)
		header.Add(HeaderErrorType, stringTypeName)
		header.Add(HeaderContentType, "text/plain")
		return target.Write(header, []byte(datum.GetError().Message))

	case *model.Datum_StageRef:
		header.Add(HeaderDatumType, DatumTypeStageRef)
		header.Add(HeaderStageRef, fmt.Sprintf("%s", datum.GetStageRef().StageId))
		return target.Write(header, []byte{})

	case *model.Datum_HttpReq:
		header.Add(HeaderDatumType, DatumTypeHTTPReq)
		httpReq := datum.GetHttpReq()
		for _, datumHeader := range httpReq.Headers {
			header.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}
		methodString := strings.ToUpper(model.HTTPMethod_name[int32(httpReq.Method)])

		header.Add(HeaderMethod, methodString)

		blob := httpReq.Body
		if blob != nil {
			header.Add(HeaderContentType, blob.ContentType)
		}

		if blob != nil {
			addBlobHeaders(header, blob)
		}

		return target.Write(header, []byte{})

	case *model.Datum_HttpResp:
		header.Add(HeaderDatumType, DatumTypeHTTPResp)
		httpResp := datum.GetHttpResp()
		for _, datumHeader := range httpResp.Headers {
			header.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}

		header.Add(HeaderResultCode, fmt.Sprintf("%d", httpResp.StatusCode))

		blob := httpResp.Body
		if blob != nil {
			header.Add(HeaderContentType, blob.ContentType)

		}

		if blob != nil {
			addBlobHeaders(header, blob)
		}

		return target.Write(header, []byte{})
	case *model.Datum_State:
		header.Add(HeaderDatumType, DatumTypeState)
		stateType := model.StateDatumType_name[int32(datum.GetState().Type)]
		header.Add(HeaderStateType, stateType)
		// not part of protocol, but avoids problems with having an empty body
		return target.Write(header, []byte(stateType))

	default:
		return fmt.Errorf("unsupported datum type")

	}
}

func addBlobHeaders(header textproto.MIMEHeader, blob *model.BlobDatum) {
	header.Add(HeaderContentType, blob.ContentType)
	header.Add(HeaderBlobID, blob.BlobId)
	header.Add(HeaderBlobLength, fmt.Sprintf("%d", blob.Length))
}

// WriteResult emits a result to an HTTP part
func WriteResult(target WriteTarget, result *model.CompletionResult) error {

	var status string
	if result.Successful {
		status = ResultStatusSuccess
	} else {
		status = ResultStatusFailure
	}

	header := textproto.MIMEHeader{}
	header.Add(HeaderResultStatus, status)
	return writeDatumToTarget(target, header, result.Datum)
}
