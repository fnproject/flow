package model

import (
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
)

// WritePartFromDatum emits a datum to an HTTP part
func WritePartFromDatum(store BlobStore, datum *Datum, writer *multipart.Writer) error {

	switch datum.Val.(type) {
	case *Datum_Blob:
		blob := datum.GetBlob()
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeBlob)
		h.Add(headerContentType, blob.ContentType)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		data, err := store.ReadBlob(blob)
		if err != nil {
			return err
		}
		partWriter.Write(data)
		return nil

	case *Datum_Empty:
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeEmpty)
		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		return nil
	case *Datum_Error:
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeError)
		h.Add(headerContentType, "text/plain")

		errorType := ErrorDatumType_name[int32(datum.GetError().Type)]
		stringTypeName := strings.Replace(errorType, "_", "-", -1)
		h.Add(headerErrorType, stringTypeName)
		partWriter, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		partWriter.Write([]byte(datum.GetError().Message))
		return nil
	case *Datum_StageRef:
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeStageRef)
		h.Add(headerStageRef, fmt.Sprintf("%s", datum.GetStageRef().StageRef))
		_, err := writer.CreatePart(h)
		if err != nil {
			return err
		}
		return nil
	case *Datum_HttpReq:
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeHttpReq)
		httpreq := datum.GetHttpReq()
		for _, datumHeader := range httpreq.Headers {
			h.Add(fmt.Sprintf("%s%s", headerHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}
		methodString := strings.ToUpper(HttpMethod_name[int32(httpreq.Method)])

		h.Add(headerMethod, methodString)

		blob := httpreq.Body
		if blob != nil {
			h.Add(headerContentType, blob.ContentType)

		}
		pw, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		if blob != nil {
			data, err := store.ReadBlob(blob)
			if err != nil {
				return err
			}
			pw.Write(data)
		}

		return nil
	case *Datum_HttpResp:
		h := textproto.MIMEHeader{}
		h.Add(headerDatumType, datumTypeHttpResp)
		httpresp := datum.GetHttpResp()
		for _, datumHeader := range httpresp.Headers {
			h.Add(fmt.Sprintf("%s%s", headerHeaderPrefix, datumHeader.Key), datumHeader.Value)
		}

		h.Add(headerResultCode, fmt.Sprintf("%d", httpresp.StatusCode))

		blob := httpresp.Body
		if blob != nil {
			h.Add(headerContentType, blob.ContentType)

		}
		pw, err := writer.CreatePart(h)
		if err != nil {
			return err
		}

		if blob != nil {
			data, err := store.ReadBlob(blob)
			if err != nil {
				return err
			}
			pw.Write(data)
		}

		return nil
	default:
		return fmt.Errorf("unsupported datum type")

	}
}
