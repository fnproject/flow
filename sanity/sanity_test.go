package sanity

import (
	"fmt"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/cluster"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/protocol"
	"github.com/fnproject/flow/server"
	"github.com/fnproject/flow/sharding"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
	//	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/require"
)

func TestGraphCreation(t *testing.T) {
	tc := NewCase("Graph Creation")
	tc.Call("Works ", http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
	tc.Call("Rejects Missing function ID", http.MethodPost, "/graph").ExpectServerErr(server.ErrInvalidFunctionID)
	tc.Call("Rejects Invalid function ID", http.MethodPost, "/graph?functionId=foo").ExpectServerErr(server.ErrInvalidFunctionID)
	tc.Run(t, testServer)
}

var testServer = NewTestServer()

func TestSupply(t *testing.T) {
	tc := NewCase("Supply")
	tc.StartWithGraph("Creates node").
		ThenPOST("/graph/:graphID/supply").WithHeader("content-type", "foo/bar").WithBodyString("foo").
		ExpectStageCreated()

	tc.StartWithGraph("Supply requires content type").
		ThenPOST("/graph/:graphID/supply").WithBodyString("foo").
		ExpectRequestErr(protocol.ErrMissingContentType)

	tc.StartWithGraph("Supply requires non-empty body ").
		ThenPOST("/graph/:graphID/supply").WithHeader("content-type", "foo/bar").
		ExpectServerErr(server.ErrMissingBody)

	StageAcceptsMetadata(func(s string) *APICmd {
		return tc.StartWithGraph(s).ThenPOST("/graph/:graphID/supply").WithHeader("content-type", "foo/bar").WithBodyString("foo")
	})

	tc.Run(t, testServer)
}

func TestCompletedValue(t *testing.T) {
	tc := NewCase("Completed Value")

	f := func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/completedValue").
			WithHeader("fnproject-resultstatus", "success")
	}

	StageAcceptsBlobType(f)
	StageAcceptsErrorType(f)
	StageAcceptsEmptyType(f)
	StageAcceptsHTTPReqType(f)
	StageAcceptsHTTPRespType(f)

	StageAcceptsMetadata(func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/completedValue").
			WithHeader("fnproject-resultstatus", "success").
			With(emptyDatumInRequest)
	})

	tc.Run(t, testServer)
}

func TestDirectCompletion(t *testing.T) {
	tc := NewCase("Direct Completion")

	tc.StartWithGraph("Creates External Completion").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated()

	tc.StartWithGraph("Completes External Completion without status fails").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated().
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		ExpectRequestErr(protocol.ErrMissingResultStatus)

	tc.StartWithGraph("Completes External Completion with invalid  status fails").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated().
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		WithHeaders(map[string]string{
		protocol.HeaderResultStatus: "baah",
	}).ExpectRequestErr(protocol.ErrInvalidResultStatus)

	tc.StartWithGraph("Completes External Completion Successfully").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated().
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		WithHeaders(map[string]string{"fnproject-resultstatus": "success",}).
		ExpectStatus(200).
		ExpectLastStageEvent(func(ctx *testCtx, msg model.Event) {
		evt, ok := msg.(*model.StageCompletedEvent)
		require.True(ctx, ok)
		assert.True(ctx, evt.Result.Successful)
	})

	tc.StartWithGraph("Completes External Completion With Failure").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated().
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		WithHeaders(map[string]string{"fnproject-resultstatus": "failure",}).
		ExpectStatus(200).
		ExpectLastStageEvent(func(ctx *testCtx, msg model.Event) {
		evt, ok := msg.(*model.StageCompletedEvent)
		require.True(ctx, ok)
		assert.False(ctx, evt.Result.Successful)
	})

	tc.StartWithGraph("Conflicts on already completed stage ").
		ThenPOST("/graph/:graphID/externalCompletion").
		ExpectStageCreated().
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		WithHeaders(map[string]string{"fnproject-resultstatus": "failure"}).
		ThenPOST("/graph/:graphID/stage/:stageID/complete").
		With(emptyDatumInRequest).
		WithHeaders(map[string]string{
		"fnproject-resultstatus": "failure",
	}).ExpectStatus(409)

	StageAcceptsMetadata(func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/externalCompletion")
	})

	f := func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/externalCompletion").
			ExpectStageCreated().
			ThenPOST("/graph/:graphID/stage/:stageID/complete").
			WithHeaders(map[string]string{"fnproject-resultstatus": "success"})
	}

	StageAcceptsBlobType(f)
	StageAcceptsErrorType(f)
	StageAcceptsEmptyType(f)
	StageAcceptsHTTPReqType(f)
	StageAcceptsHTTPRespType(f)

	tc.Run(t, testServer)
}

func TestInvokeFunction(t *testing.T) {
	tc := NewCase("Invoke Function")

	tc.StartWithGraph("Works Without Body").
		ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "GET", "fnproject-header-foo": "bar"}).
		ExpectStageCreated()

	tc.StartWithGraph("Works With Body").
		ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "POST", "fnproject-header-foo": "bar", "content-type": "text/plain"}).WithBodyString("input").
		ExpectStageCreated()

	tc.Run(t, testServer)

	tc.StartWithGraph("Rejects non-httpreq datum").
		ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "blob", "fnproject-method": "GET"}).WithBodyString("input").
		ExpectRequestErr(protocol.ErrInvalidDatumType)

	tc.StartWithGraph("Rejects missing functionId").
		ThenPOST("/graph/:graphID/invokeFunction").
		ExpectRequestErr(server.ErrInvalidFunctionID)

	StageAcceptsMetadata(func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo").
			WithHeaders(map[string]string{"fnproject-datumtype": "httpreq", "fnproject-method": "GET", "fnproject-header-foo": "bar"})
	})

	tc.Run(t, testServer)
}

func TestDelay(t *testing.T) {
	tc := NewCase("Delay Call")

	tc.StartWithGraph("Works").
		ThenPOST("/graph/:graphID/delay?delayMs=5").
		ExpectStageCreated()

	tc.StartWithGraph("Rejects Negative Delay").
		ThenPOST("/graph/:graphID/delay?delayMs=-5").
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	tc.StartWithGraph("Rejects Large delay").
		ThenPOST(fmt.Sprintf("/graph/:graphID/delay?delayMs=%d", 3600*1000*24+1)).
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	tc.StartWithGraph("Rejects missing delay").
		ThenPOST("/graph/:graphID/delay?delayMs").
		ExpectRequestErr(server.ErrMissingOrInvalidDelay)

	StageAcceptsMetadata(func(s string) *APICmd {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/delay?delayMs=5")
	})

	tc.Run(t, testServer)
}

func NewTestServer() *server.Server {

	blobStorage := persistence.NewInMemBlobStore()
	persistenceProvider := persistence.NewInMemoryProvider(1000)
	clusterSettings := &cluster.Settings{
		NodeCount:  1,
		NodeID:     0,
		NodePrefix: "node-",
		NodePort:   8081,
	}
	shardExtractor := sharding.NewFixedSizeExtractor(10 * clusterSettings.NodeCount)
	clusterManager := cluster.NewManager(clusterSettings, shardExtractor)
	shards := clusterManager.LocalShards()
	graphManager, err := actor.NewGraphManager(persistenceProvider, blobStorage, "http:", shardExtractor, shards)
	if err != nil {
		panic(err)
	}
	s, err := server.New(clusterManager, graphManager, blobStorage, ":8081", 1*time.Second, "")
	if err != nil {
		panic(err)
	}
	return s
}

func StageAcceptsBlobType(s func(string) *APICmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)
	s("Rejects missing content type").WithBodyString("str").WithHeader("fnproject-datumtype", "blob").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingContentType)
	s("Accepts valid blob datum").WithBodyString("str").WithBlobDatum("content/type", "body").ExpectStageCreated()

}

func StageAcceptsErrorType(s func(string) *APICmd) {

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

func StageAcceptsEmptyType(s func(string) *APICmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts empty datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "empty",
		"content-type":        "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}

func StageAcceptsHTTPReqType(s func(string) *APICmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts httpreq datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype": "httpreq",
		"fnproject-method":    "get",

		"content-type": "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}

func StageAcceptsHTTPRespType(s func(string) *APICmd) {

	s("Rejects missing datum type").WithBodyString("str").WithHeader("content-type", "content/type").WithBodyString("body").ExpectRequestErr(protocol.ErrMissingDatumType)

	s("Accepts httpresp datum").
		WithBodyString("str").
		WithHeaders(map[string]string{
		"fnproject-datumtype":  "httpresp",
		"fnproject-resultcode": "100",
		"content-type":         "text/plain"}).
		WithBodyString("body").ExpectStageCreated()

}

func StageAcceptsMetadata(s func(string) *APICmd) {

	s("Works with no metadata ").
		ExpectStageCreated().
		ExpectLastStageAddedEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
		assert.Equal(ctx, "", event.CodeLocation)
		assert.Equal(ctx, "", event.CallerId)
	})

	s("Works with no metadata ").
		WithHeader(protocol.HeaderCodeLocation, "code-loc").
		WithHeader(protocol.HeaderCallerRef, "caller-id").
		ExpectStageCreated().
		ExpectLastStageAddedEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
		assert.Equal(ctx, "code-loc", event.CodeLocation)
		assert.Equal(ctx, "caller-id", event.CallerId)

	})

}

func emptyDatumInRequest(cmd *APICmd) {
	cmd.WithHeader("fnproject-datumtype", "empty")
}
