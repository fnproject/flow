package actor

import (
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"
	"bytes"
	"io/ioutil"
	"github.com/sirupsen/logrus"
	"github.com/fnproject/completer/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	mock.Mock
}

var testlog = logrus.New()

func (mock *MockClient) Do(req *http.Request) (*http.Response, error) {
	args := mock.Called(req)
	resp,err :=args.Get(0).(*http.Response), args.Error(1)
	testlog.Infof("mock called %v, return (%v,%v)", req,resp,err)
	return resp,err
}

func TestShouldInvokeStageNormally(t *testing.T) {
	m := &MockClient{}

	// Note headers names have to be well-formed here.
	resp := &http.Response{
		StatusCode: 200,
		Header: map[string][]string{
			"Content-Type":           {"response/type"},
			"Fnproject-Resultstatus": {"success"},
			"Fnproject-Datumtype":    {"blob"}},
		Body: ioutil.NopCloser(bytes.NewReader([]byte("ResultBytes"))),
	}

	m.On("Do", mock.AnythingOfType("*http.Request")).Return(resp, nil)

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

	assert.Equal(t, "/function/id/", result.FunctionId)
	assert.Equal(t, "stage-id", result.StageId)
	assert.Equal(t, "graph-id", result.GraphId)
	require.NotNil(t, result.Result)
	assert.True(t, result.Result.Successful)
	require.NotNil(t, result.Result.GetDatum())
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
