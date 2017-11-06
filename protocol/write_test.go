package protocol

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldWriteSimpleBlob(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("foo"))
	datum := &model.Datum{Val: &model.Datum_Blob{Blob: blob}}

	headers, body := writeDatum(store, datum)

	assert.Equal(t, headers.Get(HeaderDatumType), DatumTypeBlob)
	assert.Equal(t, "text/plain", headers.Get(HeaderContentType))
	assert.Equal(t, "foo", body)

}

func TestShouldWriteEmptyBlob(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte(""))

	datum := &model.Datum{Val: &model.Datum_Blob{Blob: blob}}

	_, body := writeDatum(store, datum)
	assert.Equal(t, "", body)
}

func TestShouldWriteEmptyDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeEmpty, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Equal(t, "", body)
}

func TestShouldWriteErrorDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Error{Error: &model.ErrorDatum{Message: "error", Type: model.ErrorDatumType_function_timeout}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeError, headers.Get(HeaderDatumType))
	assert.Equal(t, "function-timeout", headers.Get(HeaderErrorType))
	assert.Equal(t, body, "error")
}

func TestShouldWriteStageRefDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_StageRef{StageRef: &model.StageRefDatum{StageRef: "3141"}}}
	store := persistence.NewInMemBlobStore()

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeStageRef, headers.Get(HeaderDatumType))
	assert.Equal(t, "3141", headers.Get(HeaderStageRef))
	assert.Empty(t, body)
}

func TestShouldWriteHttpReqDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("test"))

	datum := &model.Datum{Val: &model.Datum_HttpReq{
		HttpReq: &model.HTTPReqDatum{
			Method: model.HTTPMethod_options,
			Headers: []*model.HTTPHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: blob,
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeHTTPReq, headers.Get(HeaderDatumType))
	assert.Equal(t, "OPTIONS", headers.Get(HeaderMethod))

	assert.Equal(t, "text/plain", headers.Get(HeaderContentType))
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
		HttpReq: &model.HTTPReqDatum{
			Method:  model.HTTPMethod_get,
			Headers: []*model.HTTPHeader{},
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeHTTPReq, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Empty(t, body)
}

func TestShouldWriteHttpRespDatum(t *testing.T) {

	store := persistence.NewInMemBlobStore()
	blob, _ := store.CreateBlob("text/plain", []byte("test"))

	datum := &model.Datum{Val: &model.Datum_HttpResp{
		HttpResp: &model.HTTPRespDatum{
			StatusCode: 401,
			Headers: []*model.HTTPHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: blob,
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeHTTPResp, headers.Get(HeaderDatumType))
	assert.Equal(t, "401", headers.Get(HeaderResultCode))

	assert.Equal(t, "text/plain", headers.Get(HeaderContentType))
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
		HttpResp: &model.HTTPRespDatum{
			StatusCode: 401,
			Headers:    []*model.HTTPHeader{},
		}},
	}

	headers, body := writeDatum(store, datum)
	assert.Equal(t, DatumTypeHTTPResp, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Empty(t, body)
}

func TestShouldAddResultStatusToResult(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	blob, err := store.CreateBlob("content/type", []byte("content"))
	require.NoError(t, err)

	headers, _ := writeResult(store, model.NewSuccessfulResult(model.NewBlobDatum(blob)))
	assert.Equal(t, ResultStatusSuccess, headers.Get(HeaderResultStatus))

	headers, _ = writeResult(store, model.NewFailedResult(model.NewBlobDatum(blob)))
	assert.Equal(t, ResultStatusFailure, headers.Get(HeaderResultStatus))

}

func TestShouldWriteStateDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	stateDatum := model.NewStateDatum(model.StateDatumType_succeeded)

	headers, body := writeDatum(store, stateDatum)

	assert.Equal(t, DatumTypeState, headers.Get(HeaderDatumType))
	assert.Equal(t, "succeeded", headers.Get(HeaderStateType))
	assert.Equal(t, "succeeded", body)

}

func writeResult(store persistence.BlobStore, result *model.CompletionResult) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	err := WritePartFromResult(store, result, pw)
	if err != nil {
		panic(err)
	}
	pw.Close()
	headers, body := extractPart(buf, pw.Boundary())
	return headers, body
}

func writeDatum(store persistence.BlobStore, datum *model.Datum) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	err := WritePartFromDatum(store, datum, pw)
	if err != nil {
		panic(err)
	}
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
