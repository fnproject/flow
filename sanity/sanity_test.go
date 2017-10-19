package sanity

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/fnproject/completer/sharding"

	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/cluster"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/protocol"
	"github.com/fnproject/completer/server"
	"github.com/stretchr/testify/assert"
)

func TestGraphCreation(t *testing.T) {
	tc := NewCase("Graph Creation")
	tc.Call("Works ", http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
	tc.Call("Rejects Missing function ID", http.MethodPost, "/graph").ExpectServerErr(server.ErrInvalidFunctionId)
	tc.Call("Rejects Invalid function ID", http.MethodPost, "/graph?functionId=foo").ExpectServerErr(server.ErrInvalidFunctionId)
	tc.Run(t, testServer)
}

var testServer = NewTestServer()

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

	tc.StartWithGraph("Accepts code location and persists it to event").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithBodyString("foo").WithHeader("content-type", "foo/bar").WithHeader("FnProject-CodeLoc", "fn-2187").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			fmt.Sprint("checking %v", event)
			assert.Equal(ctx, "fn-2187", event.CodeLocation)
		})
	tc.StartWithGraph("Accepts CallerId and persists it to event").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithBodyString("foo").WithHeader("content-type", "foo/bar").
		WithHeader(protocol.HeaderCallerRef, "1").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			fmt.Sprint("checking %v", event)
			assert.Equal(ctx, "1", event.CallerId)
		})
	tc.StartWithGraph("Does not fail with no CallerId").
		ThenCall(http.MethodPost, "/graph/:graphId/supply").WithBodyString("foo").WithHeader("content-type", "foo/bar").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			fmt.Sprint("checking %v", event)
			assert.Equal(ctx, "", event.CallerId)
		})

	tc.Run(t, testServer)
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

	tc.StartWithGraph("Simple codelocation test").
		ThenCall(http.MethodPost, "/graph/:graphId/completedValue").WithBodyString("str").
		WithHeaders(map[string]string{
			"fnproject-datumtype": "empty",
			"content-type":        "text/plain"}).WithHeader(protocol.HeaderCodeLocation, "fn-2187").
		WithBodyString("body").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "fn-2187", event.CodeLocation)
		})
	tc.StartWithGraph("Simple CallerId test").
		ThenCall(http.MethodPost, "/graph/:graphId/completedValue").WithBodyString("str").
		WithHeaders(map[string]string{
			"fnproject-datumtype": "empty",
			"content-type":        "text/plain"}).WithHeader(protocol.HeaderCallerRef, "7").
		WithBodyString("body").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "7", event.CallerId)
		})
	tc.StartWithGraph("Empty CallerId noFail test").
		ThenCall(http.MethodPost, "/graph/:graphId/completedValue").WithBodyString("str").
		WithHeaders(map[string]string{
			"fnproject-datumtype": "empty",
			"content-type":        "text/plain"}).
		WithBodyString("body").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "", event.CallerId)
		})

	tc.Run(t, testServer)
}

func TestExternalCompletion(t *testing.T) {
	tc := NewCase("Completed Value")

	tc.StartWithGraph("Creates External Completion").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").
		ExpectStageCreated()

	tc.StartWithGraph("Completes External Completion Successfully").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").
		ExpectStageCreated().
		ThenCall(http.MethodPost, "/graph/:graphId/stage/:stageId/complete").ExpectStatus(200)

	tc.StartWithGraph("Fails External Completion Successfully").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").
		ExpectStageCreated().
		ThenCall(http.MethodPost, "/graph/:graphId/stage/:stageId/fail").
		ExpectStatus(200)

	tc.StartWithGraph("Creates External Completion to test CodeLocation").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").WithHeader(protocol.HeaderCodeLocation, "fn-2187").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "fn-2187", event.CodeLocation)
		})
	tc.StartWithGraph("Creates External Completion to test CallerId").
		ThenCall(http.MethodPost, "/graph/:graphId/externalCompletion").WithHeader(protocol.HeaderCallerRef, "allTheWorld").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "allTheWorld", event.CallerId)
		})

	tc.Run(t, testServer)
}

func TestInvokeFunction(t *testing.T) {
	tc := NewCase("Invoke Function")

	tc.StartWithGraph("Works Without Body").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "GET", "fnproject-header-foo": "bar"}).
		ExpectStageCreated()

	tc.StartWithGraph("Works With Body").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "POST", "fnproject-header-foo": "bar", "content-type": "text/plain"}).WithBodyString("input").
		ExpectStageCreated()

	tc.Run(t, testServer)

	tc.StartWithGraph("Rejects non-httpreq datum").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "blob", "fnproject-method": "GET"}).WithBodyString("input").
		ExpectRequestErr(protocol.ErrInvalidDatumType)

	tc.StartWithGraph("Rejects missing functionId").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction").
		ExpectRequestErr(server.ErrInvalidFunctionId)

	tc.StartWithGraph("Works With CodeLocation").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "GET", "fnproject-header-foo": "bar"}).
		WithHeader(protocol.HeaderCodeLocation, "fn-2187").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "fn-2187", event.CodeLocation)
		})
	tc.StartWithGraph("Works With Callerid").
		ThenCall(http.MethodPost, "/graph/:graphId/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "GET", "fnproject-header-foo": "bar"}).
		WithHeader(protocol.HeaderCallerRef, "thrust").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "thrust", event.CallerId)
		})

	tc.Run(t, testServer)
}

func TestDelay(t *testing.T) {
	tc := NewCase("Delay Call")

	tc.StartWithGraph("Works").
		ThenCall(http.MethodPost, "/graph/:graphId/delay?delayMs=5").
		ExpectStageCreated()

	tc.StartWithGraph("Rejects Negative Delay").
		ThenCall(http.MethodPost, "/graph/:graphId/delay?delayMs=-5").
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	tc.StartWithGraph("Rejects Large delay").
		ThenCall(http.MethodPost, fmt.Sprintf("/graph/:graphId/delay?delayMs=%d", 3600*1000*24+1)).
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	tc.StartWithGraph("Rejects missing delay").
		ThenCall(http.MethodPost, "/graph/:graphId/delay?delayMs").
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	tc.StartWithGraph("CodeLocation works").
		ThenCall(http.MethodPost, "/graph/:graphId/delay?delayMs=5").
		WithHeader(protocol.HeaderCodeLocation, "fn-2187").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "fn-2187", event.CodeLocation)
		})
	tc.StartWithGraph("CallerId works").
		ThenCall(http.MethodPost, "/graph/:graphId/delay?delayMs=5").
		WithHeader(protocol.HeaderCallerRef, "Bristol").
		ExpectStageCreated().
		ExpectLastStageEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "Bristol", event.CallerId)
		})

	tc.Run(t, testServer)
}

func NewTestServer() *server.Server {

	blobStorage := persistence.NewInMemBlobStore()
	persistenceProvider := persistence.NewInMemoryProvider(1000)
	clusterSettings := &cluster.ClusterSettings{
		NodeCount:  1,
		NodeID:     0,
		NodePrefix: "node-",
	}
	shardExtractor := sharding.NewFixedSizeExtractor(10 * clusterSettings.NodeCount)
	clusterManager := cluster.NewManager(clusterSettings, shardExtractor)
	graphManager, err := actor.NewGraphManager(persistenceProvider, blobStorage, "http:", shardExtractor)
	if err != nil {
		panic(err)
	}
	s, err := server.New(clusterManager, graphManager, blobStorage, ":8081", 1*time.Second)
	if err != nil {
		panic(err)
	}
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
