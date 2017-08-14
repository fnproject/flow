package model

import (
	"testing"
	"mime/multipart"
	"bytes"
	"net/textproto"
	"github.com/stretchr/testify/assert"
)

func TestRejectsUnrecognisedType(t *testing.T) {
	h := emptyHeaders()
	h.Add(headerDatumType, "unknown")
	part := createPart("p1", h, "")

	_, err := DatumFromPart(part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unrecognised datum type")

}

func TestRejectsDatumWithoutTypeHeader(t *testing.T) {
	part := createPart("p1", emptyHeaders(), "")
	_, err := DatumFromPart(part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "the "+headerDatumType+" is not present")
}

func TestRejectsBlobDatumWithNoContentType(t *testing.T) {
	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeBlob)
	part := createPart("p1", h, "")
	_, err := DatumFromPart(part)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Content-Type header")

}

func TestReadsEmptyBlobDatum(t *testing.T) {
	h := emptyHeaders()
	h.Add(headerDatumType, datumTypeBlob)
	h.Add(headerContentType, "text/plain")
	part := createPart("p1", h, "")
	d, err := DatumFromPart(part)

	assert.NoError(t, err)
	if !assert.NotNil(t, d.GetBlob()) {
		return
	}
	assert.Equal(t, d.GetBlob().ContentType, "text/plain")
	assert.Equal(t, d.GetBlob().DataString, []byte(""))
}
func TestReadsBlobHeaders(t *testing.T) {}
func TestReadsBlobDatum(t *testing.T)   {}

func TestReadErrorType(t *testing.T)             {}
func TestReadErrorTypeEmptyBody(t *testing.T)    {}
func TestReadErrorTypeUnknownError(t *testing.T) {}

func emptyHeaders() textproto.MIMEHeader {
	return textproto.MIMEHeader(make(map[string][]string))
}
func createPart(filename string, headers textproto.MIMEHeader, content string) *multipart.Part {
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
