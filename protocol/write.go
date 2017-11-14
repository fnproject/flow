package protocol

import (
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/fnproject/flow/model"
)

func writePartFromDatum(h textproto.MIMEHeader, datum *model.Datum, writer *multipart.Writer) error {
	switch datum.Val.(type) {
	case *model.Datum_Blob:
		blob := datum.GetBlob()
		h.Add(HeaderDatumType, DatumTypeBlob)
		writeBlobHeaders(h, blob)
		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		return nil

	case *model.Datum_Empty:
		h.Add(HeaderDatumType, DatumTypeEmpty)
		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		return nil
	case *model.Datum_Error:
		h.Add(HeaderDatumType, DatumTypeError)
		errorType := model.ErrorDatumType_name[int32(datum.GetError().Type)]
		stringTypeName := strings.Replace(errorType, "_", "-", -1)
		h.Add(HeaderErrorType, stringTypeName)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		partWriter.Write([]byte(datum.GetError().Message))
		return nil
	case *model.Datum_StageRef:
		h.Add(HeaderDatumType, DatumTypeStageRef)
		h.Add(HeaderStageRef, fmt.Sprintf("%s", datum.GetStageRef().StageRef))
		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		return nil
	case *model.Datum_HttpReq:
		h.Add(HeaderDatumType, DatumTypeHTTPReq)
		httpreq := datum.GetHttpReq()
		for _, datumHeader := range httpreq.Headers {
			h.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}
		methodString := strings.ToUpper(model.HTTPMethod_name[int32(httpreq.Method)])

		h.Add(HeaderMethod, methodString)

		blob := httpreq.Body
		if blob != nil {
			h.Add(HeaderContentType, blob.ContentType)

		}

		if blob != nil {
			writeBlobHeaders(h, blob)
		}

		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		return nil
	case *model.Datum_HttpResp:
		h.Add(HeaderDatumType, DatumTypeHTTPResp)
		httpresp := datum.GetHttpResp()
		for _, datumHeader := range httpresp.Headers {
			h.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}

		h.Add(HeaderResultCode, fmt.Sprintf("%d", httpresp.StatusCode))

		blob := httpresp.Body
		if blob != nil {
			h.Add(HeaderContentType, blob.ContentType)

		}

		if blob != nil {
			writeBlobHeaders(h, blob)
		}

		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		return nil
	case *model.Datum_State:
		h.Add(HeaderDatumType, DatumTypeState)
		stateType := model.StateDatumType_name[int32(datum.GetState().Type)]
		h.Add(HeaderStateType, stateType)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		// not part of protocol, but avoids problems with having an empty body
		partWriter.Write([]byte(stateType))

		return nil
	default:
		return fmt.Errorf("unsupported datum type")

	}
}
func writeBlobHeaders(h textproto.MIMEHeader, blob *model.BlobDatum) {
	h.Add(HeaderContentType, blob.ContentType)
	h.Add(HeaderBlobID, blob.BlobId)
	h.Add(HeaderBlobLength, fmt.Sprintf("%d", blob.Length))
}

// WritePartFromDatum emits a datum to an HTTP part
func WritePartFromDatum(datum *model.Datum, writer *multipart.Writer) error {
	return writePartFromDatum(textproto.MIMEHeader{}, datum, writer)
}

// WritePartFromResult emits a result to an HTTP part
func WritePartFromResult(result *model.CompletionResult, writer *multipart.Writer) error {
	h := textproto.MIMEHeader{}
	h.Add(HeaderResultStatus, getResultStatus(result))
	return writePartFromDatum(h, result.Datum, writer)
}

func getResultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return ResultStatusSuccess
	}
	return ResultStatusFailure
}
