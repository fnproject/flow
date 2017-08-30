package protocol

import (
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
)

func writePartFromDatum(h textproto.MIMEHeader, store persistence.BlobStore, datum *model.Datum, writer *multipart.Writer) error {
	switch datum.Val.(type) {
	case *model.Datum_Blob:
		blob := datum.GetBlob()
		h.Add(HeaderDatumType, DatumTypeBlob)
		h.Add(HeaderContentType, blob.ContentType)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		data, err := store.ReadBlobData(blob)
		if err != nil {
			return err
		}
		partWriter.Write(data)
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
		h.Add(HeaderDatumType, DatumTypeHttpReq)
		httpreq := datum.GetHttpReq()
		for _, datumHeader := range httpreq.Headers {
			h.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}
		methodString := strings.ToUpper(model.HttpMethod_name[int32(httpreq.Method)])

		h.Add(HeaderMethod, methodString)

		blob := httpreq.Body
		if blob != nil {
			h.Add(HeaderContentType, blob.ContentType)

		}
		pw, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		if blob != nil {
			data, err := store.ReadBlobData(blob)
			if err != nil {
				return err
			}
			pw.Write(data)
		}

		return nil
	case *model.Datum_HttpResp:
		h.Add(HeaderDatumType, DatumTypeHttpResp)
		httpresp := datum.GetHttpResp()
		for _, datumHeader := range httpresp.Headers {
			h.Add(fmt.Sprintf("%s%s", HeaderHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}

		h.Add(HeaderResultCode, fmt.Sprintf("%d", httpresp.StatusCode))

		blob := httpresp.Body
		if blob != nil {
			h.Add(HeaderContentType, blob.ContentType)

		}
		pw, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		if blob != nil {
			data, err := store.ReadBlobData(blob)
			if err != nil {
				return err
			}
			pw.Write(data)
		}

		return nil
	case *model.Datum_Status:
		h.Add(HeaderDatumType, DatumTypeStatus)
		statusValue := model.StatusDatumType_name[int32(datum.GetStatus().Type)]
		h.Add(HeaderStatusValue, statusValue)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		partWriter.Write([]byte(datum.GetError().Message))
		return nil
	default:
		return fmt.Errorf("unsupported datum type")

	}
}

// WritePartFromDatum emits a datum to an HTTP part
func WritePartFromDatum(store persistence.BlobStore, datum *model.Datum, writer *multipart.Writer) error {
	return writePartFromDatum(textproto.MIMEHeader{}, store, datum, writer)
}

// WritePartFromResult emits a result to an HTTP part
func WritePartFromResult(store persistence.BlobStore, result *model.CompletionResult, writer *multipart.Writer) error {
	h := textproto.MIMEHeader{}
	h.Add(HeaderResultStatus, GetResultStatus(result))
	return writePartFromDatum(h, store, result.Datum, writer)
}

func GetResultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return ResultStatusSuccess
	}
	return ResultStatusFailure
}
