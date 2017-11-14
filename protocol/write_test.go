package protocol

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/fnproject/flow/model"
	"github.com/stretchr/testify/assert"
)

func sampleBlobBody() *model.BlobDatum {
	return model.NewBlobBody("blobId", 101, "text/plain")
}

func hasSampleBlobHeaders(t *testing.T, headers textproto.MIMEHeader) {
	assert.Equal(t, "text/plain", headers.Get(HeaderContentType))
	assert.Equal(t, "blobId", headers.Get(HeaderBlobID))
	assert.Equal(t, "101", headers.Get(HeaderBlobLength))
}

func TestShouldWriteSimpleBlob(t *testing.T) {
	datum := model.NewBlobDatum(model.NewBlobBody("blobId", uint64(101), "text/plain"))

	headers, body := writeDatum(datum)

	assert.Equal(t, headers.Get(HeaderDatumType), DatumTypeBlob)
	hasSampleBlobHeaders(t, headers)
	assert.Equal(t, "", body)

}

func TestShouldWriteEmptyDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeEmpty, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Equal(t, "", body)
}

func TestShouldWriteErrorDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_Error{Error: &model.ErrorDatum{Message: "error", Type: model.ErrorDatumType_function_timeout}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeError, headers.Get(HeaderDatumType))
	assert.Equal(t, "function-timeout", headers.Get(HeaderErrorType))
	assert.Equal(t, body, "error")
}

func TestShouldWriteStageRefDatum(t *testing.T) {
	datum := &model.Datum{Val: &model.Datum_StageRef{StageRef: &model.StageRefDatum{StageRef: "3141"}}}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeStageRef, headers.Get(HeaderDatumType))
	assert.Equal(t, "3141", headers.Get(HeaderStageRef))
	assert.Empty(t, body)
}

func TestShouldWriteHttpReqDatum(t *testing.T) {

	datum := &model.Datum{Val: &model.Datum_HttpReq{
		HttpReq: &model.HTTPReqDatum{
			Method: model.HTTPMethod_options,
			Headers: []*model.HTTPHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: sampleBlobBody(),
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeHTTPReq, headers.Get(HeaderDatumType))
	assert.Equal(t, "OPTIONS", headers.Get(HeaderMethod))

	assert.Equal(t, []string{"h1val1", "h1val2"}, headers["Fnproject-Header-H1"])
	assert.Equal(t, "h2val", headers.Get("Fnproject-Header-H2"))
	val, present := headers["Fnproject-Header-Emptyheader"]
	assert.Equal(t, []string{""}, val)
	assert.True(t, present)

	hasSampleBlobHeaders(t, headers)

	assert.Equal(t, "", body)
}

func TestShouldWriteHttpReqDatumWithNoBody(t *testing.T) {

	datum := &model.Datum{Val: &model.Datum_HttpReq{
		HttpReq: &model.HTTPReqDatum{
			Method:  model.HTTPMethod_get,
			Headers: []*model.HTTPHeader{},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeHTTPReq, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Empty(t, body)
}

func TestShouldWriteHttpRespDatum(t *testing.T) {

	datum := &model.Datum{Val: &model.Datum_HttpResp{
		HttpResp: &model.HTTPRespDatum{
			StatusCode: 401,
			Headers: []*model.HTTPHeader{
				{"h1", "h1val1"},
				{"h1", "h1val2"},
				{"h2", "h2val"},

				{"EmptyHeader", ""},
			},
			Body: sampleBlobBody(),
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeHTTPResp, headers.Get(HeaderDatumType))
	assert.Equal(t, "401", headers.Get(HeaderResultCode))

	hasSampleBlobHeaders(t, headers)

	assert.Equal(t, "text/plain", headers.Get(HeaderContentType))
	assert.Equal(t, []string{"h1val1", "h1val2"}, headers["Fnproject-Header-H1"])
	assert.Equal(t, "h2val", headers.Get("Fnproject-Header-H2"))
	val, present := headers["Fnproject-Header-Emptyheader"]
	assert.Equal(t, []string{""}, val)
	assert.True(t, present)

	assert.Empty(t, body)
}

func TestShouldWriteHttpRespDatumWithNoBody(t *testing.T) {

	datum := &model.Datum{Val: &model.Datum_HttpResp{
		HttpResp: &model.HTTPRespDatum{
			StatusCode: 40,
			Headers:    []*model.HTTPHeader{},
		}},
	}

	headers, body := writeDatum(datum)
	assert.Equal(t, DatumTypeHTTPResp, headers.Get(HeaderDatumType))
	assert.Empty(t, headers.Get(HeaderContentType))
	assert.Empty(t, body)
}

func TestShouldAddResultStatusToResult(t *testing.T) {

	headers, _ := writeResult(model.NewSuccessfulResult(model.NewBlobDatum(sampleBlobBody())))
	assert.Equal(t, ResultStatusSuccess, headers.Get(HeaderResultStatus))

	headers, _ = writeResult(model.NewFailedResult(model.NewBlobDatum(sampleBlobBody())))
	assert.Equal(t, ResultStatusFailure, headers.Get(HeaderResultStatus))

}

func TestShouldWriteStateDatum(t *testing.T) {

	stateDatum := model.NewStateDatum(model.StateDatumType_succeeded)

	headers, body := writeDatum(stateDatum)

	assert.Equal(t, DatumTypeState, headers.Get(HeaderDatumType))
	assert.Equal(t, "succeeded", headers.Get(HeaderStateType))
	assert.Equal(t, "succeeded", body)

}

func writeResult(result *model.CompletionResult) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	err := WritePartFromResult(result, pw)
	if err != nil {
		panic(err)
	}
	pw.Close()
	headers, body := extractPart(buf, pw.Boundary())
	return headers, body
}

func writeDatum(datum *model.Datum) (textproto.MIMEHeader, string) {
	buf := new(bytes.Buffer)
	pw := multipart.NewWriter(buf)
	err := WritePartFromDatum(datum, pw)
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
