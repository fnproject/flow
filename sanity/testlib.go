package sanity

// sanity is a simple testing framework for flow service - it allows easy chaining of dependent calls by retaining object placeholders for previous calls

import (
	"bytes"
	"fmt"
	"github.com/fnproject/flow/model"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"strings"
)

import (
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

type testCtx struct {
	failed       bool
	narrative    []string
	t            *testing.T
	graphID      string
	stageID      string
	lastResponse *HTTPResp
	server       *server.Server
}

func (tc *testCtx) String() string {
	return strings.Join(tc.narrative, " > ")
}

func (tc *testCtx) PushNarrative(narrative string) *testCtx {
	newTc := *tc
	newTc.narrative = append(tc.narrative, narrative)
	return &newTc
}

// TestCase is a Server API test case that can be recorded and run
type TestCase struct {
	narrative string
	tests     []*APIChain
}

// NewCase starts a test case with a given name
func NewCase(narrative string) *TestCase {
	return &TestCase{narrative: narrative}
}

// HTTPResp wraps an httpResponse with a buffered body
type HTTPResp struct {
	resp *http.Response
	body []byte
}

func (r *HTTPResp) String() string {
	return fmt.Sprintf("code:%d  h: %v  body: %s", r.resp.StatusCode, r.resp.Header, string(r.body))
}

type resultFunc func(ctx *testCtx, response *HTTPResp)

type resultAction struct {
	narrative string
	action    resultFunc
}

// APIChain is an operation on root API
type APIChain struct {
	narrative string
	path      string
	method    string
	headers   map[string]string
	body      []byte
	expect    []*resultAction
	cmd       []*APIChain
}

func (tc *testCtx) Errorf(msg string, args ...interface{}) {
	tc.failed = true
	tc.t.Logf("Expectation failed: \n\t\t%s ", strings.Join(tc.narrative, "\n\t\t ->  "))
	tc.t.Logf(msg, args...)
	tc.t.Logf("Last Response was: %v", tc.lastResponse)
	tc.t.Fail()
}

func (tc *testCtx) FailNow() {
	tc.t.FailNow()
}

// With adds a middleware to a test that updates teh current command in-place
func (c *APIChain) With(op func(*APIChain)) *APIChain {
	op(c)
	return c
}

// WithHeader appends a header to the current request
func (c *APIChain) WithHeader(k string, v string) *APIChain {
	c.headers[k] = v
	return c
}

// WithHeaders appends a map of headers to the current request
func (c *APIChain) WithHeaders(h map[string]string) *APIChain {
	for k, v := range h {
		c.headers[k] = v
	}
	return c
}

// Expect appends an expectation to the current case
func (c *APIChain) Expect(fn resultFunc, msg string, args ...interface{}) *APIChain {
	c.expect = append(c.expect, &resultAction{fmt.Sprintf(msg, args...), fn})
	return c
}

// ExpectStatus creates an HTTP code expectation
func (c *APIChain) ExpectStatus(status int) *APIChain {
	return c.Expect(func(ctx *testCtx, resp *HTTPResp) {
		assert.Equal(ctx, status, resp.resp.StatusCode, "Http status should be %d", status)
	}, "status matches %d", status)
}

// ExpectGraphCreated - verifies that the server reported a graph was created
func (c *APIChain) ExpectGraphCreated() *APIChain {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *HTTPResp) {
			flowIDHeader := resp.resp.Header.Get("Fnproject-FlowId")
			require.NotEmpty(ctx, flowIDHeader, "FlowId header must be present in headers %v ", resp.resp.Header)
			ctx.graphID = flowIDHeader
		}, "Graph was created")
}

// ExpectStageCreated verifies that the server reported that  a stage was created
func (c *APIChain) ExpectStageCreated() *APIChain {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *HTTPResp) {
			stage := resp.resp.Header.Get("Fnproject-StageId")
			require.NotEmpty(ctx, stage, "StageID not in header")
			ctx.stageID = stage
		}, "Stage was created")
}

// ExpectLastStageAddedEvent  adds an expectation on the last StageAddedEvent
func (c *APIChain) ExpectLastStageAddedEvent(test func(*testCtx, *model.StageAddedEvent)) *APIChain {
	return c.Expect(func(ctx *testCtx, resp *HTTPResp) {
		var lastStageAddedEvent *model.StageAddedEvent

		ctx.server.GraphManager.QueryGraphEvents(ctx.graphID, 0,
			func(event *persistence.StreamEvent) bool {
				_, ok := event.Event.(*model.StageAddedEvent)
				return ok
			},
			func(event *persistence.StreamEvent) {
				lastStageAddedEvent = event.Event.(*model.StageAddedEvent)
			})

		require.NotNil(ctx, lastStageAddedEvent, "Expecting at least one stage added event, got none")

		test(ctx, lastStageAddedEvent)
	}, "Expecting Stage added event ")
}

// ExpectLastStageEvent  adds an expectation on the last StageMessage (of any type)
func (c *APIChain) ExpectLastStageEvent(test func(*testCtx, model.Event)) *APIChain {
	return c.Expect(func(ctx *testCtx, resp *HTTPResp) {
		var lastStageEvent model.Event

		ctx.server.GraphManager.QueryGraphEvents(ctx.graphID, 0,
			func(event *persistence.StreamEvent) bool {
				_, ok := event.Event.(model.Event)
				return ok
			},
			func(event *persistence.StreamEvent) {
				lastStageEvent = event.Event.(model.Event)
			})

		require.NotNil(ctx, lastStageEvent, "Expecting at least one stage  event, got none")

		test(ctx, lastStageEvent)
	}, "Expecting Stage message")
}

// ExpectRequestErr expects an request error matching a given error case
func (c *APIChain) ExpectRequestErr(serverErr error) *APIChain {
	return c.Expect(func(ctx *testCtx, resp *HTTPResp) {
		assert.Equal(ctx, 400, resp.resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.resp.Header.Get("content-type"))
		assert.Equal(ctx, serverErr.Error(), string(resp.body), "Error body did not match")
	}, "Request Error :  %s", serverErr.Error())
}

// ExpectServerErr expects a server-side error matching a given error
func (c *APIChain) ExpectServerErr(serverErr *server.Error) *APIChain {
	return c.Expect(func(ctx *testCtx, resp *HTTPResp) {
		assert.Equal(ctx, serverErr.HTTPStatus, resp.resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.resp.Header.Get("content-type"))
		assert.Equal(ctx, serverErr.Message, string(resp.body), "Error body did not match")
	}, "Server Error : %d %s", serverErr.HTTPStatus, serverErr.Message)
}

// WithBodyString adds a body string to the current command
func (c *APIChain) WithBodyString(data string) *APIChain {
	c.body = []byte(data)
	return c

}

// WithBlobDatum appends a blob datum to the current request
func (c *APIChain) WithBlobDatum(contentType string, data string) *APIChain {
	return c.WithBodyString(data).WithHeaders(map[string]string{
		"content-type":        contentType,
		"fnproject-datumtype": "blob",
	})

}

// WithErrorDatum appends an ErrorDatum to the current request
func (c *APIChain) WithErrorDatum(errorType string, message string) *APIChain {
	return c.WithBodyString(message).WithHeaders(map[string]string{
		"content-type":        "text/plain",
		"fnproject-errortype": errorType,
		"fnproject-datumtype": "error",
	})
}

// ThenCall chains a new api-call on to the state of the previous call - placeholders (e.g. :stageID, :graphID)  inherited from the previous call will be substituted into the next path
func (c *APIChain) ThenCall(method string, path string) *APIChain {
	newCmd := &APIChain{method: method, path: path, headers: map[string]string{}}
	c.cmd = append(c.cmd, newCmd)
	return newCmd
}

// ThenPOST is a shorthand for c.ThenCall("POST",path)
func (c *APIChain) ThenPOST(path string) *APIChain {
	return c.ThenCall("POST", path)
}

func (c *APIChain) toReq(ctx *testCtx) *http.Request {
	placeholders := func(key string) string {

		var s = strings.Replace(key, ":graphID", ctx.graphID, -1)
		s = strings.Replace(s, ":stageID", ctx.stageID, -1)
		return s
	}
	headers := map[string][]string{}

	for k, v := range c.headers {
		headers[http.CanonicalHeaderKey(placeholders(k))] = []string{placeholders(v)}
	}

	u, err := url.Parse(placeholders(c.path))
	require.NoError(ctx, err, " invalid URL ")

	r := &http.Request{
		URL:    u,
		Method: c.method,
		Header: headers,
	}
	if c.body != nil {
		r.Body = ioutil.NopCloser(bytes.NewReader(c.body))
	} else {
		r.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
	}
	return r
}

func (c *APIChain) run(ctx testCtx, s *server.Server) {
	ctx.server = s
	nuCtx := (&ctx).PushNarrative(c.narrative)

	req := c.toReq(nuCtx)
	resp := httptest.NewRecorder()

	nuCtx = nuCtx.PushNarrative(fmt.Sprintf("%s %s", req.Method, req.URL))
	fmt.Printf("Test : %s\n", nuCtx)

	s.Engine.ServeHTTP(resp, req)
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(ctx.t, err)

	nuCtx.lastResponse = &HTTPResp{resp: resp.Result(), body: buf.Bytes()}

	for _, check := range c.expect {
		nuCtx = nuCtx.PushNarrative(check.narrative)
		check.action(nuCtx, nuCtx.lastResponse)
	}

	for _, cmd := range c.cmd {
		cmd.run(*nuCtx, s)
	}

}

// Call Starts a test tree with an arbitrary HTTP call
func (c *TestCase) Call(description string, method string, path string) *APIChain {
	cmd := &APIChain{narrative: c.narrative + ":" + description, method: method, path: path, headers: map[string]string{}}
	c.tests = append(c.tests, cmd)
	return cmd
}

// StartWithGraph creates a new test tree with an graph
func (c *TestCase) StartWithGraph(description string) *APIChain {
	return c.Call(description, http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
}

// Run runs an whole test tree.
func (c *TestCase) Run(t *testing.T, server *server.Server) {

	for _, tc := range c.tests {
		t.Run(tc.narrative, func(t *testing.T) {
			ctx := testCtx{t: t}
			tc.run(ctx, server)
		})

	}
}
