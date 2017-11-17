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
	"github.com/fnproject/flow/blobs"
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
		ThenPOST("/graph/:graphID/supply").
		With(validClosure).
		ExpectStageCreated()

	StageAcceptsRawBlob(func(s string) *APIChain { return tc.StartWithGraph(s).ThenPOST("/graph/:graphID/supply") })

	StageAcceptsMetadata(func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/supply").
			With(validClosure)
	})

	tc.Run(t, testServer)
}

func TestCompletedValue(t *testing.T) {
	tc := NewCase("Completed Value")

	StageAcceptsAllBlobTypes(func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/completedValue").
			WithHeader("fnproject-resultstatus", "success")
	})

	StageAcceptsMetadata(func(s string) *APIChain {
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
		WithHeaders(map[string]string{"fnproject-resultstatus": "success"}).
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
		WithHeaders(map[string]string{"fnproject-resultstatus": "failure"}).
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

	StageAcceptsMetadata(func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/externalCompletion")
	})

	f := func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/externalCompletion").
			ExpectStageCreated().
			ThenPOST("/graph/:graphID/stage/:stageID/complete").
			WithHeaders(map[string]string{"fnproject-resultstatus": "success"})
	}

	StageAcceptsAllBlobTypes(f)

	tc.Run(t, testServer)
}

func TestInvokeFunction(t *testing.T) {
	tc := NewCase("Invoke Function")

	StageAcceptsHTTPReqType(func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo")
	})

	tc.Run(t, testServer)

	tc.StartWithGraph("Rejects non-httpreq datum").
		ThenPOST("/graph/:graphID/invokeFunction?functionId=fn/foo").
		WithHeaders(map[string]string{"fnproject-datumtype": "blob", "fnproject-method": "GET"}).WithBodyString("input").
		ExpectRequestErr(protocol.ErrInvalidDatumType)

	tc.StartWithGraph("Rejects missing functionId").
		ThenPOST("/graph/:graphID/invokeFunction").
		ExpectRequestErr(server.ErrInvalidFunctionID)

	StageAcceptsMetadata(func(s string) *APIChain {
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

	StageAcceptsMetadata(func(s string) *APIChain {
		return tc.StartWithGraph(s).
			ThenPOST("/graph/:graphID/delay?delayMs=5")
	})

	tc.Run(t, testServer)
}

func NewTestServer() *server.Server {
	blobStorage := blobs.NewInMemBlobStore()
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
	s, err := server.New(clusterManager, graphManager, ":8081", 1*time.Second, "")
	if err != nil {
		panic(err)
	}
	return s
}

func validClosure(a *APIChain) {
	a.WithHeaders(map[string]string{"Content-Type": "text/plain",
		"Fnproject-BlobId":     "BlobId",
		"Fnproject-BlobLength": "100"})

}

func StageAcceptsRawBlob(s func(string) *APIChain) {
	var requiredBlobHeaders = map[string]headerExpectation{
		"Content-type":         {good: "text/plain", missingError: protocol.ErrMissingContentType},
		"Fnproject-blobId":     {good: "blobId", missingError: protocol.ErrMissingBlobID},
		"Fnproject-blobLength": {good: "100", missingError: protocol.ErrMissingBlobLength, bad: "foo", invalidError: protocol.ErrInvalidBlobLength}}

	StageHonorsHeaderReqs("Raw Blob", s, requiredBlobHeaders)
}

func StageAcceptsAllBlobTypes(f func(s string) *APIChain) {
	StageAcceptsBlobDatum(f)
	StageAcceptsErrorDatum(f)
	StageAcceptsEmptyType(f)
	StageAcceptsHTTPReqType(f)
	StageAcceptsHTTPRespType(f)
}
func StageAcceptsBlobDatum(s func(string) *APIChain) {
	StageHonorsHeaderReqs("Blob Datum", s, map[string]headerExpectation{
		"FnProject-datumtype":  {good: "blob", missingError: protocol.ErrMissingDatumType},
		"Content-type":         {good: "text/plain", missingError: protocol.ErrMissingContentType},
		"Fnproject-blobId":     {good: "blobId", missingError: protocol.ErrMissingBlobID},
		"Fnproject-blobLength": {good: "100", missingError: protocol.ErrMissingBlobLength, bad: "foo", invalidError: protocol.ErrInvalidBlobLength},
	})

}

func StageAcceptsErrorDatum(s func(string) *APIChain) {

	StageHonorsHeaderReqs("Error Datum", s, map[string]headerExpectation{
		"FnProject-datumtype": {good: "error", missingError: protocol.ErrMissingDatumType},
		"Content-type":        {good: "text/plain", missingError: protocol.ErrMissingContentType, bad: "application/xml", invalidError: protocol.ErrInvalidContentType},
		"Fnproject-errortype": {good: "error", missingError: protocol.ErrMissingErrorType},
	})

}

func StageAcceptsEmptyType(s func(string) *APIChain) {
	StageHonorsHeaderReqs("Empty Datum ", s, map[string]headerExpectation{
		"FnProject-datumtype": {good: "empty", missingError: protocol.ErrMissingDatumType},
	})
}

func StageAcceptsHTTPReqType(s func(string) *APIChain) {

	StageHonorsHeaderReqs("HttpReq (No blob)", s, map[string]headerExpectation{
		"FnProject-datumtype": {good: "httpreq", missingError: protocol.ErrMissingDatumType},
		"Fnproject-Method":    {good: "POST", missingError: protocol.ErrMissingHTTPMethod, bad: "wibble", invalidError: protocol.ErrInvalidHTTPMethod},
	})

	StageHonorsHeaderReqs("HttpReq (with blob)", s, map[string]headerExpectation{
		"FnProject-datumtype":  {good: "httpreq", missingError: protocol.ErrMissingDatumType},
		"Fnproject-Method":     {good: "POST"},
		"Fnproject-blobId":     {good: "blobId"},
		"Content-type":         {good: "text/plain", missingError: protocol.ErrMissingContentType},
		"Fnproject-blobLength": {good: "100", missingError: protocol.ErrMissingBlobLength, bad: "foo", invalidError: protocol.ErrInvalidBlobLength},
	})

}

func StageAcceptsHTTPRespType(s func(string) *APIChain) {

	StageHonorsHeaderReqs("HttpResp (No blob)", s, map[string]headerExpectation{
		"FnProject-datumtype":  {good: "httpresp", missingError: protocol.ErrMissingDatumType},
		"Fnproject-resultcode": {good: "100", missingError: protocol.ErrMissingResultCode, bad: "wibble", invalidError: protocol.ErrInvalidResultCode},
	})

	StageHonorsHeaderReqs("HTTPResp (with blob)", s, map[string]headerExpectation{
		"FnProject-datumtype":  {good: "httpresp", missingError: protocol.ErrMissingDatumType},
		"Fnproject-resultcode": {good: "100"},
		"Fnproject-blobId":     {good: "blobId"},
		"Content-type":         {good: "text/plain", missingError: protocol.ErrMissingContentType},
		"Fnproject-blobLength": {good: "100", missingError: protocol.ErrMissingBlobLength, bad: "foo", invalidError: protocol.ErrInvalidBlobLength}})

}

func StageAcceptsMetadata(s func(string) *APIChain) {

	s("Works with no metadata").
		ExpectStageCreated().
		ExpectLastStageAddedEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Empty(ctx, event.CodeLocation)
			assert.Empty(ctx, event.CallerId)
		})

	s("Works with metadata").
		WithHeader(protocol.HeaderCodeLocation, "code-loc").
		WithHeader(protocol.HeaderCallerRef, "caller-id").
		ExpectStageCreated().
		ExpectLastStageAddedEvent(func(ctx *testCtx, event *model.StageAddedEvent) {
			assert.Equal(ctx, "code-loc", event.CodeLocation)
			assert.Equal(ctx, "caller-id", event.CallerId)

		})

}

func emptyDatumInRequest(cmd *APIChain) {
	cmd.WithHeader("fnproject-datumtype", "empty")
}

type headerExpectation struct {
	good         string
	bad          string
	missingError error
	invalidError error
}

// StageHonorsHeaderReqs determines if a call accepts and requires  specific headers it generates a case for each missing and invalid header described in the header expectations
func StageHonorsHeaderReqs(caseName string, g func(s string) *APIChain, exectations map[string]headerExpectation) {

	for h, he := range exectations {
		if he.missingError != nil {
			newHeaders := map[string]string{}
			for oh, ve := range exectations {
				if oh != h {
					newHeaders[oh] = ve.good
				}
			}

			g(caseName + ": Missing " + h + " should raise error").WithHeaders(newHeaders).ExpectRequestErr(he.missingError)

		}

		if he.invalidError != nil {
			newHeaders := map[string]string{}
			for oh, ve := range exectations {
				if oh != h {
					newHeaders[oh] = ve.good
				} else {
					newHeaders[oh] = ve.bad
				}
			}

			g(caseName + ": Missing " + h + " should raise error").WithHeaders(newHeaders).ExpectRequestErr(he.invalidError)
		}
	}

	goodHeaders := map[string]string{}
	for h, he := range exectations {
		goodHeaders[h] = he.good
	}
	g(caseName + " accepts good headers").WithHeaders(goodHeaders).ExpectStageCreated()
}
