package protocol

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"mime/multipart"
	"net/textproto"
	"testing"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/model"
)

func TestShouldWriteSimpleBlob(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("foo"))
	datum := &model.Datum{Val: &model.Datum_Blob{blob}}

	headers, body := writeDatum(store, datum)

	assert.Equal(t, headers.Get(headerDatumType), datumTypeBlob)
	assert.Equal(t, "text/plain", headers.Get(headerContentType))
	assert.Equal(t, "foo", body)

}

func TestShouldWriteEmptyBlob(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte(""))

	datum := &model.Datum{Val: &model.Datum_Blob{blob}}

	_, body := writeDatum(store, datum)
	assert.Equal(t, "", body)
}

func TestShouldWriteEmptyDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Empty{&model.EmptyDatum{}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, datumTypeEmpty, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Equal(t, "", body)
}

func TestShouldWriteErrorDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Error{&model.ErrorDatum{Message: "error", Type: model.ErrorDatumType_function_timeout}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, datumTypeError, headers.Get(headerDatumType))
	assert.Equal(t, "function-timeout", headers.Get(headerErrorType))
	assert.Equal(t, "text/plain", headers.Get(headerContentType))

	assert.Equal(t, body, "error")
}

func TestShouldWriteStageRefDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_StageRef{&model.StageRefDatum{StageRef: "3141"}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, datumTypeStageRef, headers.Get(headerDatumType))
	assert.Equal(t, "3141", headers.Get(headerStageRef))
	assert.Empty(t, body)
}

func TestShouldWriteHttpReqDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("test"))

	datum := &model.Datum{Val: &model.Datum_HttpReq{
		HttpReq: &model.HttpReqDatum{
			Method: model.HttpMethod_options,
			Headers: []*model.HttpHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: blob,
		}},
	}

	headers, body := writeDatum(store, datum)
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
	store := persistence.NewInMemBlobStore()

	datum := &model.Datum{Val: &model.Datum_HttpReq{
		HttpReq: &model.HttpReqDatum{
			Method:  model.HttpMethod_get,
			Headers: []*model.HttpHeader{},
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, datumTypeHttpReq, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Empty(t, body)
}

func TestShouldWriteHttpRespDatum(t *testing.T) {

	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("test"))

	datum := &model.Datum{Val: &model.Datum_HttpResp{
		HttpResp: &model.HttpRespDatum{
			StatusCode: 401,
			Headers: []*model.HttpHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: blob,
		}},
	}

	headers, body := writeDatum(store, datum)
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
	store := persistence.NewInMemBlobStore()

	datum := &model.Datum{Val: &model.Datum_HttpResp{
		HttpResp: &model.HttpRespDatum{
			StatusCode: 401,
			Headers:    []*model.HttpHeader{},
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, datumTypeHttpResp, headers.Get(headerDatumType))
	assert.Empty(t, headers.Get(headerContentType))
	assert.Empty(t, body)
}

func writeDatum(store persistence.BlobStore, datum *model.Datum) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	WritePartFromDatum(store, datum, pw)
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
