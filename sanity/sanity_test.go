package sanity

import (
	"net/http"
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/fnproject/completer/server"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/actor"

	"github.com/gin-gonic/gin"
	"github.com/fnproject/completer/protocol"
	"github.com/fnproject/completer/model"
)

func TestGraphCreation(t *testing.T) {
	tc := NewCase("Graph Creation")
	tc.Call("Works ", http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
	tc.Call("Rejects Missing function ID", http.MethodPost, "/graph").ExpectServerErr(server.ErrInvalidFunctionId)
	tc.Call("Rejects Invalid function ID", http.MethodPost, "/graph?functionId=foo").ExpectServerErr(server.ErrInvalidFunctionId)
	tc.Run(t, NewTestServer(t))
}

func TestSupply(t *testing.T) {
	tc := NewCase("Supply")
	tc.StartWithGraph("Creates node").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithHeader("content-type", "foo/bar").WithBodyString("foo").
		ExpectStageCreated()

	tc.StartWithGraph("Supply requires content type").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithBodyString("foo").
		ExpectRequestErr(protocol.ErrMissingContentType)

	tc.StartWithGraph("Supply requires non-empty body ").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithHeader("content-type", "foo/bar").
		ExpectServerErr(server.ErrMissingBody)

	tc.Run(t, NewTestServer(t))
}


func TestCompletedValue(t *testing.T) {
	tc := NewCase("Completed Value")

	f := func(s string) *apiCmd {
		return tc.StartWithGraph(s).
			ThenCall(http.MethodPost, "/graph/:graphId/completedValue")
	}

	StageAcceptsBlobType(f)
	StageAcceptsErrorType(f)
	StageAcceptsEmptyType(f)
	StageAcceptsHttpReqType(f)
	StageAcceptsHttpRespType(f)

	tc.Run(t, NewTestServer(t))
}


func TestExternalCompletion(t *testing.T) {
	tc := NewCase("Completed Value")
	tc.StartWithGraph("Creates External Completion").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").ExpectStageCreated()
	
	tc.StartWithGraph("Completes External Completion Successfully").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").ExpectStageCreated().
		ThenCall(http.MethodPost, "/graph/:graphId/stage/:stageId/complete").ExpectStatus(200)

	tc.StartWithGraph("Fails External Completion Successfully").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").ExpectStageCreated().
		ThenCall(http.MethodPost, "/graph/:graphId/stage/:stageId/fail").ExpectStatus(200)


	tc.Run(t, NewTestServer(t))
}

func NewTestServer(t *testing.T) *server.Server {
	gin.SetMode(gin.ReleaseMode)

	blobStorage := persistence.NewInMemBlobStore()
	persistenceProvider := persistence.NewInMemoryProvider(1000)
	graphManager, err := actor.NewGraphManager(persistenceProvider, blobStorage, "http:///")
	require.NoError(t, err)

	s, err := server.New(graphManager, blobStorage, ":8081")
	require.NoError(t, err)
	return s

}




func StageAcceptsBlobType(s func(string) *apiCmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)
	s("Rejects missing content type").WithBodyString("str").WithHeader("fnproject-datumtype", "blob").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingContentType)
	s("Accepts valid blob datum").WithBodyString("str").WithBlobDatum("content/type", "body").ExpectStageCreated()

}

func StageAcceptsErrorType(s func(string) *apiCmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Rejects missing error type").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "error",
		"content-type":        "text/plain"}).
		WithBodyString("body").ExpectRequestErr(protocol.ErrMissingErrorType)

	s("Rejects missing content type").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "error",
		"fnproject-errortype": "error"}).
		WithBodyString("body").ExpectRequestErr(protocol.ErrMissingContentType)

	s("Rejects non-text content type").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "error",
		"fnproject-errortype": "error",
		"content-type":        "application/octet-stream"}).
		WithBodyString("body").ExpectRequestErr(protocol.ErrInvalidContentType)

	s("Accepts valid error datum").WithBodyString("str").WithErrorDatum(model.ErrorDatumType_name[int32(model.ErrorDatumType_invalid_stage_response)], "msg").ExpectStageCreated()
	s("Accepts invalid error type ").WithBodyString("str").WithErrorDatum("XXX foo  Error", "msg").ExpectStageCreated()

}

func StageAcceptsEmptyType(s func(string) *apiCmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts empty datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "empty",
		"content-type":        "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}

func StageAcceptsHttpReqType(s func(string) *apiCmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts httpreq datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "httpreq",
		"fnproject-method":    "get",

		"content-type": "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}

func StageAcceptsHttpRespType(s func(string) *apiCmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts httpresp datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype":  "httpresp",
		"fnproject-resultcode": "100",
		"content-type":         "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}