package protocol

import (
	"bytes"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
)

func TestRejectsUnrecognisedType(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, "unknown")
	part := createEmptyPart(h)

	_, err := DatumFromPart(part)
	assert.Equal(t, ErrInvalidDatumType, err)

}

func TestRejectsDatumWithoutTypeHeader(t *testing.T) {

	part := createEmptyPart(emptyHeaders())
	_, err := DatumFromPart( part)
	assert.Equal(t, ErrMissingDatumType, err)

}

func TestRejectsBlobDatumWithNoContentType(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeBlob)
	part := createEmptyPart(h)
	_, err := DatumFromPart( part)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingContentType, err)

}

// TODO invalid blobId invalid BlobLength

func TestReadsEmptyBlobDatum(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeBlob)
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderBlobID, "blobId")
	h.Add(HeaderBlobLength, "101")

	part := createEmptyPart(h)
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetBlob())
	assert.Equal(t, "text/plain", d.GetBlob().ContentType)
	assert.Equal(t, "blobId", d.GetBlob().BlobId)
	assert.Equal(t, uint64(101), d.GetBlob().Length)
}


func TestReadErrorDatumAllTypes(t *testing.T) {
	for _, errName := range model.ErrorDatumType_name {

		h := emptyHeaders()
		h.Add(HeaderDatumType, DatumTypeError)
		h.Add(HeaderContentType, "text/plain")
		h.Add(HeaderErrorType, errName)
		part := createPart(h, "blah")
		d, err := DatumFromPart( part)

		assert.NoError(t, err)
		require.NotNil(t, d.GetError())
		assert.Equal(t, errName, d.GetError().GetType().String())
		assert.Equal(t, "blah", d.GetError().GetMessage())
	}
}

func TestRejectsErrorDatumIfNotTextPlain(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeError)
	h.Add(HeaderContentType, "application/json")
	h.Add(HeaderErrorType, "unknown_error")
	part := createPart(h, "")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidContentType, err)
}

func TestRejectsErrorDatumIfNoErrorType(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeError)
	h.Add(HeaderContentType, "text/plain")
	part := createPart(h, "")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrMissingErrorType, err)
}

func TestRejectsErrorDatumIfNoContentType(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeError)
	h.Add(HeaderErrorType, "unknown_error")
	part := createPart(h, "")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrMissingContentType, err)
}

func TestReadErrorTypeEmptyBody(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeError)
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderErrorType, "unknown_error")
	part := createPart(h, "")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetError())
	assert.Equal(t, "unknown_error", d.GetError().GetType().String())
	assert.Equal(t, "", d.GetError().GetMessage())
}

func TestReadErrorTypeUnrecognizedErrorIsCoercedToUnknownError(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeError)
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderErrorType, "LA LA LA PLEASE IGNORE ME LA LA LA")
	part := createPart(h, "blah")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetError())
	assert.Equal(t, "unknown_error", d.GetError().GetType().String())
	assert.Equal(t, "blah", d.GetError().GetMessage())
}

func TestReadsStageRefDatum(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeStageRef)
	h.Add(HeaderStageRef, "123")
	part := createPart(h, "")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetStageRef())
	assert.Equal(t, "123", d.GetStageRef().GetStageRef())
}

func TestRejectsStageRefDatumWithNoStageRef(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeStageRef)
	h.Add(HeaderStageRef, "")
	part := createPart(h, "")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrMissingStageRef, err)
}

func TestReadsHttpReqDatumWithBodyAndHeaders(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPReq)
	h.Add(HeaderMethod, "GET")
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderBlobID, "blobId")
	h.Add(HeaderBlobLength, "101")

	part := createPart(h, "WOMBAT")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetHttpReq())
	assert.Equal(t, model.HTTPMethod_get, d.GetHttpReq().GetMethod())

	require.NotNil(t, d.GetHttpReq().GetBody())
	body := d.GetHttpReq().GetBody()
	assert.Equal(t, "text/plain", body.ContentType)
	assert.Equal(t, "blobId", body.BlobId)
	assert.Equal(t, uint64(101), body.Length)

	require.Equal(t, 3, len(d.GetHttpReq().GetHeaders()))
	multiHeaders := d.GetHttpReq().GetHeaderValues("Multi")
	require.Equal(t, 2, len(multiHeaders))
	assert.Equal(t, "BAR", multiHeaders[0])
	assert.Equal(t, "BAZ", multiHeaders[1])
	singleHeader := d.GetHttpReq().GetHeaderValues("Single")
	require.Equal(t, 1, len(singleHeader))
	assert.Equal(t, "FOO", singleHeader[0])
}

func TestReadsHttpReqDatumWithNoBody(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPReq)
	h.Add(HeaderMethod, "GET")
	part := createPart(h, "")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetHttpReq())
	assert.Equal(t, model.HTTPMethod_get, d.GetHttpReq().GetMethod())

	assert.Nil(t, d.GetHttpReq().Body)

}

func TestRejectsHttpReqDatumWithNoMethod(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPReq)
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")

	part := createPart(h, "WOMBAT")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrMissingHTTPMethod, err)
}

func TestRejectsHttpReqDatumWithInvalidMethod(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPReq)
	h.Add(HeaderMethod, "SOME_INVALID_METHOD")
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")
	part := createPart(h, "WOMBAT")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidHTTPMethod, err)
}

func TestReadsHttpRespDatumWithBodyAndHeaders(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPResp)
	h.Add(HeaderResultCode, "200")
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderBlobID, "blobId")
	h.Add(HeaderBlobLength, "101")
	part := createPart(h, "WOMBAT")
	d, err := DatumFromPart( part)

	assert.NoError(t, err)
	require.NotNil(t, d.GetHttpResp())
	assert.Equal(t, uint32(200), d.GetHttpResp().GetStatusCode())
	assert.Equal(t, "text/plain", d.GetHttpResp().GetBody().ContentType)

	require.NotNil(t, d.GetHttpResp().GetBody())
	body := d.GetHttpResp().GetBody()
	assert.Equal(t, "text/plain", body.ContentType)
	assert.Equal(t, "blobId", body.BlobId)
	assert.Equal(t, uint64(101), body.Length)

	require.Equal(t, 3, len(d.GetHttpResp().GetHeaders()))
	multiHeaders := d.GetHttpResp().GetHeaderValues("Multi")
	require.Equal(t, 2, len(multiHeaders))
	assert.Equal(t, "BAR", multiHeaders[0])
	assert.Equal(t, "BAZ", multiHeaders[1])
	singleHeader := d.GetHttpResp().GetHeaderValues("Single")
	require.Equal(t, 1, len(singleHeader))
	assert.Equal(t, "FOO", singleHeader[0])
}

func TestReadsSuccessfulEncapsulatedResultFromHttpResp(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := http.Header{}
	h.Add(HeaderDatumType, DatumTypeBlob)
	h.Add(HeaderResultCode, "200")
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderResultStatus, ResultStatusSuccess)
	h.Add(HeaderBlobID, "blobId")
	h.Add(HeaderBlobLength, "101")

	content := []byte("content")
	innerResp := &http.Response{

		Header:     h,
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader(content)),
	}

	buf := bytes.Buffer{}

	err := innerResp.Write(&buf)
	require.NoError(t, err)

	outerResp := &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader(buf.Bytes())),
	}

	result, err := CompletionResultFromEncapsulatedResponse(store, outerResp)

	require.NoError(t, err)

	blob := result.GetDatum().GetBlob()

	assert.Equal(t, "text/plain", blob.ContentType)
	assert.Equal(t, "blobId", blob.BlobId)
	assert.Equal(t, uint64(101), blob.Length)

	assert.True(t, result.Successful)


}

func TestReadsFailedncapsulatedResultFromHttpResp(t *testing.T) {
	store := persistence.NewInMemBlobStore()

	h := http.Header{}
	h.Add(HeaderDatumType, DatumTypeBlob)
	h.Add(HeaderResultCode, "200")
	h.Add(HeaderContentType, "text/plain")
	h.Add(HeaderResultStatus, ResultStatusFailure)
	h.Add(HeaderBlobID, "blobId")
	h.Add(HeaderBlobLength, "101")

	content := []byte("content")
	innerResp := &http.Response{

		Header:     h,
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader(content)),
	}

	buf := bytes.Buffer{}

	err := innerResp.Write(&buf)
	require.NoError(t, err)

	outerResp := &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader(buf.Bytes())),
	}

	result, err := CompletionResultFromEncapsulatedResponse(store, outerResp)

	require.NoError(t, err)

	blob := result.GetDatum().GetBlob()

	assert.Equal(t, "text/plain", blob.ContentType)
	assert.Equal(t, "blobId", blob.BlobId)
	assert.Equal(t, uint64(101), blob.Length)


	assert.False(t, result.Successful)

}

func TestRejectsHttpRespDatumWithNoResultCode(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPResp)
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")
	part := createPart(h, "WOMBAT")
	_, err := DatumFromPart(part)

	assert.Error(t, err)
	assert.Equal(t, ErrMissingResultCode, err)
}

func TestRejectsHttpReqDatumWithInvalidResultCode(t *testing.T) {

	h := emptyHeaders()
	h.Add(HeaderDatumType, DatumTypeHTTPResp)
	h.Add(HeaderResultCode, "SOME_INVALID_CODE")
	h.Add(HeaderHeaderPrefix+"single", "FOO")
	h.Add(HeaderHeaderPrefix+"multi", "BAR")
	h.Add(HeaderHeaderPrefix+"multi", "BAZ")
	h.Add(HeaderContentType, "text/plain")
	part := createPart(h, "WOMBAT")
	_, err := DatumFromPart( part)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidResultCode, err)
}

func emptyHeaders() textproto.MIMEHeader {
	return textproto.MIMEHeader(make(map[string][]string))
}

func createEmptyPart(headers textproto.MIMEHeader,) *multipart.Part {
	wbuf := new(bytes.Buffer)
	w := multipart.NewWriter(wbuf)
	_, err := w.CreatePart(headers)
	if err != nil {
		panic(err)
	}

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


func createPart(headers textproto.MIMEHeader,content string ) *multipart.Part {
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

