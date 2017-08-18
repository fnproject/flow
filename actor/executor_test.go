package actor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	mock.Mock
}

var testlog = logrus.New()

func (mock *MockClient) Do(req *http.Request) (*http.Response, error) {
	args := mock.Called(req)
	resp, err := args.Get(0), args.Error(1)
	if resp != nil {
		return resp.(*http.Response), err
	}
	return nil, err
}

func givenEncapsulatedResponse(statusCode int, headers http.Header, body []byte) *http.Response {
	buf := &bytes.Buffer{}
	encap := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Header:     headers,
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
	}
	encap.Write(buf)
	log.Info(buf.String())
	return &http.Response{
		StatusCode: statusCode,
		Header:     map[string][]string{},
		Body:       ioutil.NopCloser(buf),
	}
}
func TestShouldInvokeStageNormally(t *testing.T) {
	m := &MockClient{}

	// Note headers names have to be well-formed here.
	resp := givenEncapsulatedResponse(200,
		map[string][]string{
			"Content-Type":           {"response/type"},
			"Fnproject-Resultstatus": {"success"},
			"Fnproject-Datumtype":    {"blob"}},
		[]byte("ResultBytes"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(m)

	hasValidResult(t, result)
	assert.True(t, result.Result.Successful)
	require.NotNil(t, result.Result.GetDatum().GetBlob())
	blob := result.Result.GetDatum().GetBlob()
	assert.Equal(t, "response/type", blob.ContentType)
	assert.Equal(t, []byte("ResultBytes"), blob.DataString)

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "POST", outbound.Method)
	assert.Contains(t, outbound.Header.Get("Content-type"), "multipart/form-data; boundary=")
	assert.Equal(t, "graph-id", outbound.Header.Get("Fnproject-threadid"))
	assert.Equal(t, "stage-id", outbound.Header.Get("Fnproject-stageid"))

}

func TestShouldHandleHttpStageError(t *testing.T) {
	m := &MockClient{}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, fmt.Errorf("error"))

	result := givenValidInvokeStageRequest(m)

	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_failed)

}

func TestShouldHandleFnTimeout(t *testing.T) {
	m := &MockClient{}

	resp := givenEncapsulatedResponse(504,
		map[string][]string{},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(m)

	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_timeout)

}

func TestShouldHandleInvalidStageResponseWithoutHeaders(t *testing.T) {
	m := &MockClient{}

	resp := givenEncapsulatedResponse(200,
		map[string][]string{},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(m)

	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_invalid_stage_response)

}

func TestShouldHandleFailedStageResponse(t *testing.T) {
	m := &MockClient{}

	resp := givenEncapsulatedResponse(500,
		map[string][]string{},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(m)
	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_failed)

}

func TestShouldInvokeFunctionNormally(t *testing.T) {
	m := &MockClient{}

	resp := &http.Response{
		StatusCode: 201,
		Header: map[string][]string{
			"Content-Type": {"response/type"},
			"RHeader_1":    {"h1val"},
			"RHeader_2":    {"h2val1", "h2val2"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidFunctionRequest(m, model.NewBlob("body/type", []byte("body")))

	hasValidResult(t, result)
	assert.True(t, result.Result.Successful)

	datum := hasValidHTTPRespResult(t, result, 201)

	blob := datum.Body
	assert.Equal(t, "response/type", blob.ContentType)
	assert.Equal(t, []byte("ResultBytes"), blob.DataString)

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "PUT", outbound.Method)
	assert.Equal(t, "body/type", outbound.Header.Get("Content-type"))
	br := &bytes.Buffer{}
	br.ReadFrom(outbound.Body)
	assert.Equal(t, []byte("body"), br.Bytes())
}

func TestShouldInvokeWithNoOutboundsBody(t *testing.T) {
	m := &MockClient{}

	resp := &http.Response{
		StatusCode: 201,
		Header: map[string][]string{
			"Content-Type": {"response/type"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	givenValidFunctionRequest(m, nil)

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "PUT", outbound.Method)
	assert.Equal(t, "", outbound.Header.Get("Content-type"))

	br := &bytes.Buffer{}
	br.ReadFrom(outbound.Body)
	assert.Equal(t, []byte(""), br.Bytes())
}

func TestShouldHandleFunctionNetworkError(t *testing.T) {
	m := &MockClient{}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, fmt.Errorf("error"))

	result := givenValidFunctionRequest(m, nil)
	hasErrorResult(t, result, model.ErrorDatumType_function_invoke_failed)

}

func TestConvertNonSuccessfulCodeToFailedStatus(t *testing.T) {
	m := &MockClient{}

	resp := &http.Response{
		StatusCode: 401,
		Header: map[string][]string{
			"Content-Type": {"response/type"},
			"RHeader_1":    {"h1val"},
			"RHeader_2":    {"h2val1", "h2val2"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}
	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidFunctionRequest(m, nil)
	hasValidHTTPRespResult(t, result, 401)
	assert.False(t, result.Result.Successful)

}

func TestResponseDefaultsToApplicationOctetStream(t *testing.T) {
	m := &MockClient{}

	resp := &http.Response{
		StatusCode: 200,
		Header: map[string][]string{
			"RHeader_1": {"h1val"},
			"RHeader_2": {"h2val1", "h2val2"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}
	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidFunctionRequest(m, nil)
	datum := hasValidHTTPRespResult(t, result, 200)

	assert.Equal(t, "application/octet-stream", datum.Body.ContentType)

}

func hasValidHTTPRespResult(t *testing.T, result *model.FaasInvocationResponse, code uint32) *model.HttpRespDatum {
	require.NotNil(t, result.Result.GetDatum().GetHttpResp())

	datum := result.Result.GetDatum().GetHttpResp()
	assert.Equal(t, code, datum.StatusCode)
	assert.Equal(t, "h1val", datum.GetHeader("RHeader_1"))
	assert.Equal(t, []string{"h2val1", "h2val2"}, datum.GetHeaderValues("RHeader_2"))
	return datum
}

func givenValidInvokeStageRequest(m *MockClient) *model.FaasInvocationResponse {
	exec := &graphExecutor{
		client:   m,
		faasAddr: "http://faasaddr",
		log:      testlog.WithField("Test", "logger"),
	}
	result := exec.HandleInvokeStageRequest(&model.InvokeStageRequest{
		GraphId:    "graph-id",
		StageId:    "stage-id",
		FunctionId: "/function/id/",
		Operation:  model.CompletionOperation_thenApply,
		Closure:    model.NewBlob("closure/type", []byte("closure")),
		Args:       []*model.Datum{model.NewBlobDatum(model.NewBlob("arg1/type", []byte("arg1"))), model.NewEmptyDatum()},
	})
	return result
}

func givenValidFunctionRequest(m *MockClient, body *model.BlobDatum) *model.FaasInvocationResponse {
	exec := &graphExecutor{
		client:   m,
		faasAddr: "http://faasaddr",
		log:      testlog.WithField("Test", "logger"),
	}
	result := exec.HandleInvokeFunctionRequest(&model.InvokeFunctionRequest{
		GraphId:    "graph-id",
		StageId:    "stage-id",
		FunctionId: "/function/id/",
		Arg: &model.HttpReqDatum{
			Method: model.HttpMethod_put,
			Body:   body,
			Headers: []*model.HttpHeader{
				{Key: "header_1", Value: "h1val"},
				{Key: "header_2", Value: "h2val_1"},
				{Key: "header_2", Value: "h2val_2"},
			},
		},
	})
	return result
}

func hasValidResult(t *testing.T, result *model.FaasInvocationResponse) {
	assert.Equal(t, "/function/id/", result.FunctionId)
	assert.Equal(t, "stage-id", result.StageId)
	assert.Equal(t, "graph-id", result.GraphId)
	require.NotNil(t, result.Result)
	require.NotNil(t, result.Result.GetDatum())

}
func hasErrorResult(t *testing.T, result *model.FaasInvocationResponse, errType model.ErrorDatumType) {
	assert.False(t, result.Result.Successful)
	require.NotNil(t, result.Result.GetDatum())
	require.NotNil(t, result.Result.GetDatum().GetError())
	errorDatum := result.Result.GetDatum().GetError()
	assert.Equal(t, errType, errorDatum.Type)
}
