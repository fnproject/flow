package actor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/fnproject/flow/blobs"
	"github.com/fnproject/flow/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/textproto"
	"github.com/gogo/protobuf/jsonpb"
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

func TestShouldInvokeStageNormally(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resultDatum := model.NewBlobDatum(model.NewBlob("blobID", 100, "response/type"))
	marshaller := jsonpb.Marshaler{}
	resultDatumJSON, _ := marshaller.MarshalToString(resultDatum)

	resp := givenResponse(200,
		map[string][]string{
			"Fn_call_id":   {"CALLID"},
			"Content-Type": {"application/json"},
		},
		[]byte(`{"result": {"successful": true, "datum": ` + resultDatumJSON + ` }}`))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(store, m)

	hasValidResult(t, result)
	assert.Equal(t, result.CallId, "CALLID")
	assert.True(t, result.Result.Successful)
	require.NotNil(t, result.Result.GetDatum().GetBlob())
	blob := result.Result.GetDatum().GetBlob()
	assert.Equal(t, "blobID", blob.BlobId)
	assert.Equal(t, int64(100), blob.Length)
	assert.Equal(t, "response/type", blob.ContentType)

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "POST", outbound.Method)
	assert.Contains(t, outbound.Header.Get("Content-type"), "application/json")
	assert.Equal(t, "graph-id", outbound.Header.Get("Fnproject-FlowId"))
	assert.Equal(t, "stage-id", outbound.Header.Get("Fnproject-Stageid"))

}

func TestShouldHandleHttpStageError(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, fmt.Errorf("error"))

	result := givenValidInvokeStageRequest(store, m)

	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_failed)

}

func TestShouldHandleFnTimeout(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := givenResponse(504,
		map[string][]string{
			"Fn_call_id": {"CALLID"},
		},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(store, m)

	assert.Equal(t, result.CallId, "CALLID")
	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_timeout)

}

func TestShouldHandleInvalidStageResponseContent(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := givenResponse(200,
		map[string][]string{
			"Fn_call_id": {"CALLID"},
			"Content-Type": {"application/json"},
		},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(store, m)

	assert.Equal(t, result.CallId, "CALLID")
	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_invalid_stage_response)

}

func TestShouldHandleFailedStageResponse(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := givenResponse(500,
		map[string][]string{
			"Fn_call_id": {"CALLID"},
		},
		[]byte("error"))

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidInvokeStageRequest(store, m)
	assert.Equal(t, result.CallId, "CALLID")
	hasValidResult(t, result)
	hasErrorResult(t, result, model.ErrorDatumType_stage_failed)

}

func TestShouldInvokeFunctionNormally(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := &http.Response{
		StatusCode: 201,
		Header: map[string][]string{
			"Fn_call_id":   {"CALLID"},
			"Content-Type": {"response/type"},
			"RHeader_1":    {"h1val"},
			"RHeader_2":    {"h2val1", "h2val2"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	blob := createBlob(t, store, "body/type", []byte("body"))
	result := givenValidFunctionRequest(store, m, blob)

	hasValidResult(t, result)
	assert.True(t, result.Result.Successful)

	assert.Equal(t, result.CallId, "CALLID")

	datum := hasValidHTTPRespResult(t, result, 201)

	assert.Equal(t, "response/type", datum.Body.ContentType)
	assert.Equal(t, []byte("ResultBytes"), getBlobData(t, store, datum.Body))

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "PUT", outbound.Method)
	assert.Equal(t, "body/type", outbound.Header.Get("Content-type"))
	assert.Equal(t, outbound.Header.Get("header_1"), "h1val")
	assert.Equal(t, outbound.Header[textproto.CanonicalMIMEHeaderKey("header_2")], []string{"h2val_1", "h2val_2"})

	br := &bytes.Buffer{}
	br.ReadFrom(outbound.Body)
	assert.Equal(t, []byte("body"), br.Bytes())
}

func TestShouldInvokeWithNoOutboundBody(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := &http.Response{
		StatusCode: 201,
		Header: map[string][]string{
			"Content-Type": {"response/type"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	givenValidFunctionRequest(store, m, nil)

	outbound := m.Calls[0].Arguments.Get(0).(*http.Request)
	assert.Equal(t, "PUT", outbound.Method)
	assert.Equal(t, "", outbound.Header.Get("Content-type"))

	br := &bytes.Buffer{}
	br.ReadFrom(outbound.Body)
	assert.Equal(t, []byte(""), br.Bytes())
}

func TestShouldHandleFunctionNetworkError(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, fmt.Errorf("error"))

	result := givenValidFunctionRequest(store, m, nil)
	hasErrorResult(t, result, model.ErrorDatumType_function_invoke_failed)

}

func TestConvertNonSuccessfulCodeToFailedStatus(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

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

	result := givenValidFunctionRequest(store, m, nil)

	hasValidHTTPRespResult(t, result, 401)
	assert.False(t, result.Result.Successful)

}

func TestResponseDefaultsToApplicationOctetStream(t *testing.T) {
	m := &MockClient{}
	store := blobs.NewInMemBlobStore()

	resp := &http.Response{
		StatusCode: 200,
		Header: map[string][]string{
			"RHeader_1": {"h1val"},
			"RHeader_2": {"h2val1", "h2val2"},
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}
	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result := givenValidFunctionRequest(store, m, nil)
	datum := hasValidHTTPRespResult(t, result, 200)

	assert.Equal(t, "application/octet-stream", datum.Body.ContentType)

}

func hasValidHTTPRespResult(t *testing.T, result *model.FaasInvocationResponse, code int32) *model.HTTPRespDatum {
	require.NotNil(t, result.Result.GetDatum().GetHttpResp())

	datum := result.Result.GetDatum().GetHttpResp()
	assert.Equal(t, code, datum.StatusCode)
	assert.Equal(t, "h1val", datum.GetHeader("RHeader_1"))
	assert.Equal(t, []string{"h2val1", "h2val2"}, datum.GetHeaderValues("RHeader_2"))
	return datum
}

func givenResponse(statusCode int, headers http.Header, body []byte) *http.Response {
	// we ignore the inner code of the frame here
	resp := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Header:     headers,
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
	}
	return resp
}

func givenValidInvokeStageRequest(store blobs.Store, m *MockClient) *model.FaasInvocationResponse {
	exec := &graphExecutor{
		blobStore: store,
		client:    m,
		faasAddr:  "http://faasaddr",
		log:       testlog.WithField("Test", "logger"),
	}

	result := exec.HandleInvokeStage(&model.InvokeStageRequest{
		FlowId:    "graph-id",
		StageId:    "stage-id",
		FunctionId: "/function/id/",
		Closure:    model.NewBlob("closure", int64(200), "content/type"),
		Args:       []*model.CompletionResult{model.NewSuccessfulResult(model.NewBlobDatum(model.NewBlob("arg1", int64(200), "content/type"))), model.NewEmptyResult()},
	})
	return result
}

func givenValidFunctionRequest(store blobs.Store, m *MockClient, body *model.BlobDatum) *model.FaasInvocationResponse {
	exec := &graphExecutor{
		blobStore: store,
		client:    m,
		faasAddr:  "http://faasaddr",
		log:       testlog.WithField("Test", "logger"),
	}

	result := exec.HandleInvokeFunction(&model.InvokeFunctionRequest{
		FlowId:    "graph-id",
		StageId:    "stage-id",
		FunctionId: "/function/id/",
		Arg: &model.HTTPReqDatum{
			Method: model.HTTPMethod_put,
			Body:   body,
			Headers: []*model.HTTPHeader{
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
	assert.Equal(t, "graph-id", result.FlowId)
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

func getBlobData(t *testing.T, s blobs.Store, blob *model.BlobDatum) []byte {
	data, err := s.Read("graph-id", blob.BlobId)

	require.NoError(t, err)
	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(data)

	require.NoError(t, err)
	return buf.Bytes()
}

func createBlob(t *testing.T, store blobs.Store, contentType string, data []byte) *model.BlobDatum {

	blob, err := store.Create("graph-id", contentType, bytes.NewReader(data))
	require.NoError(t, err)
	return model.BlobDatumFromBlobStoreBlob(blob)
}
