package model

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestShouldWriteSimpleBlob(t *testing.T) {
	datum := &Datum{Val: &Datum_Blob{&BlobDatum{ContentType: "text/plain", DataString: []byte("foo")}}}

	headers, body := writeDatum(datum)

	assert.Equal(t, headers.Get(headerDatumType), datumTypeBlob)
	assert.Equal(t, "text/plain", headers.Get(headerContentType))
	assert.Equal(t, "foo", body)

}

func TestShouldWriteEmptyBlob(t *testing.T) {
	datum := &Datum{Val: &Datum_Blob{&BlobDatum{ContentType: "text/plain", DataString: []byte{}}}}

	_, body := writeDatum(datum)
	assert.Equal(t, "", body)
}

func TestShouldWriteEmptyDatum(t *testing.T) {
	datum := &Datum{Val: &Datum_Empty{&EmptyDatum{}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeEmpty, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Equal(t, "", body)
}

func TestShouldWriteErrorDatum(t *testing.T) {
	datum := &Datum{Val: &Datum_Error{&ErrorDatum{Message: "error", Type: ErrorDatumType_function_timeout}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeError, headers.Get(headerDatumType))
	assert.Equal(t, "function-timeout", headers.Get(headerErrorType))
	assert.Equal(t, "text/plain", headers.Get(headerContentType))

	assert.Equal(t, body, "error")
}

func TestShouldWriteStageRefDatum(t *testing.T) {
	datum := &Datum{Val: &Datum_StageRef{&StageRefDatum{StageRef: "3141"}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeStageRef, headers.Get(headerDatumType))
	assert.Equal(t, "3141", headers.Get(headerStageRef))
	assert.Empty(t, body)
}

func TestShouldWriteHttpReqDatum(t *testing.T) {
	datum := &Datum{Val: &Datum_HttpReq{
		HttpReq: &HttpReqDatum{
			Method: HttpMethod_options,
			Headers: []*HttpHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: &BlobDatum{
				ContentType: "text/plain",
				DataString:  []byte("test")},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeHttpReq, headers.Get(headerDatumType))
	assert.Equal(t, "OPTIONS", headers.Get(headerMethod))

	assert.Equal(t, "text/plain", headers.Get(headerContentType))
	assert.Equal(t, []string{"h1val1", "h1val2"}, headers["Fnproject-Header-H1"])
	assert.Equal(t, "h2val", headers.Get("Fnproject-Header-H2"))
	val, present := headers["Fnproject-Header-Emptyheader"]
	assert.Equal(t, []string{""}, val)
	assert.True(t, present)

	assert.Equal(t, "test", body)
}

func TestShouldWriteHttpReqDatumWithNoBody(t *testing.T) {
	datum := &Datum{Val: &Datum_HttpReq{
		HttpReq: &HttpReqDatum{
			Method:  HttpMethod_get,
			Headers: []*HttpHeader{},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeHttpReq, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Empty(t, body)
}

func TestShouldWriteHttpRespDatum(t *testing.T) {
	datum := &Datum{Val: &Datum_HttpResp{
		HttpResp: &HttpRespDatum{
			StatusCode: 401,
			Headers: []*HttpHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: &BlobDatum{
				ContentType: "text/plain",
				DataString:  []byte("test")},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeHttpResp, headers.Get(headerDatumType))
	assert.Equal(t, "401", headers.Get(headerResultCode))

	assert.Equal(t, "text/plain", headers.Get(headerContentType))
	assert.Equal(t, []string{"h1val1", "h1val2"}, headers["Fnproject-Header-H1"])
	assert.Equal(t, "h2val", headers.Get("Fnproject-Header-H2"))
	val, present := headers["Fnproject-Header-Emptyheader"]
	assert.Equal(t, []string{""}, val)
	assert.True(t, present)

	assert.Equal(t, "test", body)
}

func TestShouldWriteHttpRespDatumWithNoBody(t *testing.T) {
	datum := &Datum{Val: &Datum_HttpResp{
		HttpResp: &HttpRespDatum{
			StatusCode: 401,
			Headers:    []*HttpHeader{},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, datumTypeHttpResp, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Empty(t, body)
}
func writeDatum(datum *Datum) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	WritePartFromDatum(datum, pw)
	pw.Close()
	headers, body := extractPart(buf, pw.Boundary())
	return headers, body
}

func extractPart(buf *bytes.Buffer, boundary string) (textproto.MIMEHeader, string) {
	pr := multipart.NewReader(buf, boundary)

	part, err := pr.NextPart()
	if err != nil {
		panic("invalid part")
	}
	partbuf := new(bytes.Buffer)
	_, err = partbuf.ReadFrom(part)
	if err != nil {
		panic(err)
	}
	return part.Header, partbuf.String()

}
