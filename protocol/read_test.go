package protocol

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mime/multipart"
	"net/textproto"
	"testing"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/model"
)

func TestRejectsUnrecognisedType(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, "unknown")
	part := createPart( h, "")

	_, err := DatumFromPart(store, part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unrecognised datum type")

}

func TestRejectsDatumWithoutTypeHeader(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	part := createPart( emptyHeaders(), "")
	_, err := DatumFromPart(store, part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "the "+headerDatumType+" header is not present")
}

func TestRejectsBlobDatumWithNoContentType(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeBlob)
	part := createPart( h, "")
	_, err := DatumFromPart(store, part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Content-Type header")

}

func TestReadsEmptyBlobDatum(t *testing.T) {

	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeBlob)
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetBlob())
	assert.Equal(t, "text/plain", d.GetBlob().ContentType)
	assert.Equal(t, []byte(""), getBlobData(t, store, d.GetBlob()))
}

func TestReadsBlobDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeBlob)
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "SOME CONTENT")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetBlob())
	assert.Equal(t, "text/plain", d.GetBlob().ContentType)
	assert.Equal(t, []byte("SOME CONTENT"), getBlobData(t, store, d.GetBlob()))
}

func TestReadsActuallyEmptyDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeEmpty)
	part := createPart( h, "")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	assert.NotNil(t, d.GetEmpty())
}

func TestReadErrorDatumAllTypes(t *testing.T) {
	for _, errName := range model.ErrorDatumType_name {
		store := persistence.NewInMemBlobStore()

		h := emptyHeaders()
		h.Add(headerDatumType, datumTypeError)
		h.Add(headerContentType, "text/plain")
		h.Add(headerErrorType, errName)
		part := createPart( h, "blah")
		d, err := DatumFromPart(store, part)

		assert.NoError(t, err)
		require.NotNil(t, d.GetError())
		assert.Equal(t, errName, d.GetError().GetType().String())
		assert.Equal(t, "blah", d.GetError().GetMessage())
	}
}

func TestRejectsErrorDatumIfNotTextPlain(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeError)
	h.Add(headerContentType, "application/json")
	h.Add(headerErrorType, "unknown_error")
	part := createPart( h, "")
	_, err := DatumFromPart(store, part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid error datum content type")
}

func TestRejectsErrorDatumIfNoErrorType(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeError)
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "")
	_, err := DatumFromPart(store, part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no "+headerErrorType+" header defined")
}

func TestReadErrorTypeEmptyBody(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeError)
	h.Add(headerContentType, "text/plain")
	h.Add(headerErrorType, "unknown_error")
	part := createPart( h, "")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetError())
	assert.Equal(t, "unknown_error", d.GetError().GetType().String())
	assert.Equal(t, "", d.GetError().GetMessage())
}

func TestReadErrorTypeUnrecognizedErrorIsCoercedToUnknownError(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeError)
	h.Add(headerContentType, "text/plain")
	h.Add(headerErrorType, "LA LA LA PLEASE IGNORE ME LA LA LA")
	part := createPart( h, "blah")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetError())
	assert.Equal(t, "unknown_error", d.GetError().GetType().String())
	assert.Equal(t, "blah", d.GetError().GetMessage())
}

func TestReadsStageRefDatum(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeStageRef)
	h.Add(headerStageRef, "123")
	part := createPart( h, "")
	d, err := DatumFromPart(store, part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetStageRef())
	assert.Equal(t, "123", d.GetStageRef().GetStageRef())
}

func TestRejectsStageRefDatumWithNoStageRef(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeStageRef)
	h.Add(headerStageRef, "")
	part := createPart( h, "")
	_, err := DatumFromPart(store, part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid StageRef")
}

func TestReadsHttpReqDatumWithBodyAndHeaders(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpReq)
	h.Add(headerMethod, "GET")
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	d, err := DatumFromPart(store,part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetHttpReq())
	assert.Equal(t, model.HttpMethod_get, d.GetHttpReq().GetMethod())

	assert.Equal(t, "text/plain", d.GetHttpReq().GetBody().ContentType)
	assert.Equal(t, []byte("WOMBAT"), getBlobData(t, store, d.GetHttpReq().GetBody()))
	require.Equal(t, 3, len(d.GetHttpReq().GetHeaders()))
	multiHeaders := d.GetHttpReq().GetHeaderValues("Multi")
	require.Equal(t, 2, len(multiHeaders))
	assert.Equal(t, "BAR", multiHeaders[0])
	assert.Equal(t, "BAZ", multiHeaders[1])
	singleHeader := d.GetHttpReq().GetHeaderValues("Single")
	require.Equal(t, 1, len(singleHeader))
	assert.Equal(t, "FOO", singleHeader[0])
}

func TestRejectsHttpReqDatumWithNoMethod(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpReq)
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	_, err := DatumFromPart(store,part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no "+headerMethod+" header defined")
}

func TestRejectsHttpReqDatumWithInvalidMethod(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpReq)
	h.Add(headerMethod, "SOME_INVALID_METHOD")
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	_, err := DatumFromPart(store,part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http method SOME_INVALID_METHOD is invalid")
}

func TestReadsHttpRespDatumWithBodyAndHeaders(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpResp)
	h.Add(headerResultCode, "200")
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	d, err := DatumFromPart(store,part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetHttpResp())
	assert.Equal(t, uint32(200), d.GetHttpResp().GetStatusCode())
	assert.Equal(t, "text/plain", d.GetHttpResp().GetBody().ContentType)
	assert.Equal(t, []byte("WOMBAT"), getBlobData(t, store, d.GetHttpResp().GetBody()))

	require.Equal(t, 3, len(d.GetHttpResp().GetHeaders()))
	multiHeaders := d.GetHttpResp().GetHeaderValues("Multi")
	require.Equal(t, 2, len(multiHeaders))
	assert.Equal(t, "BAR", multiHeaders[0])
	assert.Equal(t, "BAZ", multiHeaders[1])
	singleHeader := d.GetHttpResp().GetHeaderValues("Single")
	require.Equal(t, 1, len(singleHeader))
	assert.Equal(t, "FOO", singleHeader[0])
}

func TestRejectsHttpRespDatumWithNoResultCode(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpResp)
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	_, err := DatumFromPart(store,part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no "+headerResultCode+" header defined")
}

func TestRejectsHttpReqDatumWithInvalidResultCode(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeHttpResp)
	h.Add(headerResultCode, "SOME_INVALID_CODE")
	h.Add(headerHeaderPrefix+"single", "FOO")
	h.Add(headerHeaderPrefix+"multi", "BAR")
	h.Add(headerHeaderPrefix+"multi", "BAZ")
	h.Add(headerContentType, "text/plain")
	part := createPart( h, "WOMBAT")
	_, err := DatumFromPart(store,part)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid HttpResp Datum :")
	assert.Contains(t, err.Error(), "parsing \"SOME_INVALID_CODE\": invalid syntax")
}

func emptyHeaders() textproto.MIMEHeader {
	return textproto.MIMEHeader(make(map[string][]string))
}
func createPart( headers textproto.MIMEHeader, content string) *multipart.Part {
	wbuf := new(bytes.Buffer)
	w := multipart.NewWriter(wbuf)
	pw, err := w.CreatePart(headers)
	if err != nil {
		panic(err)
	}
	_, err = pw.Write([]byte(content))

	if err != nil {
		panic(err)
	}

	err = w.Close()
	if err != nil {
		panic(err)
	}

	r := multipart.NewReader(wbuf, w.Boundary())
	part, err := r.NextPart()
	if err != nil {
		panic(err)
	}
	return part

}

func getBlobData(t *testing.T, store persistence.BlobStore, blob *model.BlobDatum) []byte {
	data, err := store.ReadBlobData(blob)

	require.NoError(t, err)
	return data

}
