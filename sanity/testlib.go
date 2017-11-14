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
	lastResponse *http.Response
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
	tests     []*APICmd
}

// NewCase starts a test case with a given name
func NewCase(narrative string) *TestCase {
	return &TestCase{narrative: narrative}
}

type resultFunc func(ctx *testCtx, response *http.Response)

type resultAction struct {
	narrative string
	action    resultFunc
}

// APICmd is an operation on root API
type APICmd struct {
	narrative string
	path      string
	method    string
	headers   map[string]string
	body      []byte
	expect    []*resultAction
	cmd       []*APICmd
}

func (tc *testCtx) Errorf(msg string, args ...interface{}) {
	tc.failed = true
	tc.t.Errorf("Expectation failed: \n\t\t%s ", strings.Join(tc.narrative, "\n\t\t ->  "))
	tc.t.Errorf(msg, args...)
	tc.t.Errorf("Last Response was: %v", tc.lastResponse)
}

const testFailed = "TestFailed"

func (tc *testCtx) FailNow() {
	panic(testFailed)
}

// With adds a middleware to a test that updates teh current command in-place
func (c *APICmd) With(op func(*APICmd)) *APICmd {
	op(c)
	return c
}

// WithHeader appends a header to the current request
func (c *APICmd) WithHeader(k string, v string) *APICmd {
	c.headers[k] = v
	return c
}

// WithHeaders appends a map of headers to the current request
func (c *APICmd) WithHeaders(h map[string]string) *APICmd {
	for k, v := range h {
		c.headers[k] = v
	}
	return c
}

// Expect appends an expectation to the current case
func (c *APICmd) Expect(fn resultFunc, msg string, args ...interface{}) *APICmd {
	c.expect = append(c.expect, &resultAction{fmt.Sprintf(msg, args...), fn})
	return c
}

// ExpectStatus creates an HTTP code expectation
func (c *APICmd) ExpectStatus(status int) *APICmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, status, resp.StatusCode, "Http status should be %d", status)
	}, "status matches %d", status)
}

// ExpectGraphCreated - verifies that the server reported a graph was created
func (c *APICmd) ExpectGraphCreated() *APICmd {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *http.Response) {
			flowIDHeader := resp.Header.Get("Fnproject-FlowId")
			require.NotEmpty(ctx, flowIDHeader, "FlowId header must be present in headers %v ", resp.Header)
			ctx.graphID = flowIDHeader
		}, "Graph was created")
}

// ExpectStageCreated verifies that the server reported that  a stage was created
func (c *APICmd) ExpectStageCreated() *APICmd {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *http.Response) {
			stage := resp.Header.Get("Fnproject-StageId")
			require.NotEmpty(ctx, stage, "StageID not in header")
			ctx.stageID = stage
		}, "Stage was created")
}

// ExpectLastStageAddedEvent  adds an expectation on the last StageAddedEvent
func (c *APICmd) ExpectLastStageAddedEvent(test func(*testCtx, *model.StageAddedEvent)) *APICmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
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
func (c *APICmd) ExpectLastStageEvent(test func(*testCtx, model.Event)) *APICmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
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
func (c *APICmd) ExpectRequestErr(serverErr error) *APICmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, 400, resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.Header.Get("content-type"))
		buf := &bytes.Buffer{}
		_, err := buf.ReadFrom(resp.Body)
		require.NoError(ctx, err)
		assert.Equal(ctx, serverErr.Error(), string(buf.Bytes()), "Error body did not match")
	}, "Request Error :  %s", serverErr.Error())
}

// ExpectServerErr expects a server-side error matching a given error
func (c *APICmd) ExpectServerErr(serverErr *server.Error) *APICmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, serverErr.HTTPStatus, resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.Header.Get("content-type"))
		buf := &bytes.Buffer{}
		_, err := buf.ReadFrom(resp.Body)
		require.NoError(ctx, err)
		assert.Equal(ctx, serverErr.Message, string(buf.Bytes()), "Error body did not match")
	}, "Server Error : %d %s", serverErr.HTTPStatus, serverErr.Message)
}

// WithBodyString adds a body string to the current command
func (c *APICmd) WithBodyString(data string) *APICmd {
	c.body = []byte(data)
	return c

}

// WithBlobDatum appends a blob datum to the current request
func (c *APICmd) WithBlobDatum(contentType string, data string) *APICmd {
	return c.WithBodyString(data).WithHeaders(map[string]string{
		"content-type":        contentType,
		"fnproject-datumtype": "blob",
	})

}

// WithErrorDatum appends an ErrorDatum to the current request
func (c *APICmd) WithErrorDatum(errorType string, message string) *APICmd {
	return c.WithBodyString(message).WithHeaders(map[string]string{
		"content-type":        "text/plain",
		"fnproject-errortype": errorType,
		"fnproject-datumtype": "error",
	})
}

// ThenCall chains a new api-call on to the state of the previous call - placeholders (e.g. :stageID, :graphID)  inherited from the previous call will be substituted into the next path
func (c *APICmd) ThenCall(method string, path string) *APICmd {
	newCmd := &APICmd{method: method, path: path, headers: map[string]string{}}
	c.cmd = append(c.cmd, newCmd)
	return newCmd
}

// ThenPOST is a shorthand for c.ThenCall("POST",path)
func (c *APICmd) ThenPOST(path string) *APICmd {
	return c.ThenCall("POST", path)
}

func (c *APICmd) toReq(ctx *testCtx) *http.Request {
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

func (c *APICmd) run(ctx testCtx, s *server.Server) {
	ctx.server = s
	nuCtx := (&ctx).PushNarrative(c.narrative)

	req := c.toReq(nuCtx)
	resp := httptest.NewRecorder()

	nuCtx = nuCtx.PushNarrative(fmt.Sprintf("%s %s", req.Method, req.URL))
	fmt.Printf("Test : %s\n", nuCtx)

	s.Engine.ServeHTTP(resp, req)
	nuCtx.lastResponse = resp.Result()

	for _, check := range c.expect {
		nuCtx = nuCtx.PushNarrative(check.narrative)
		check.action(nuCtx, nuCtx.lastResponse)
	}

	for _, cmd := range c.cmd {
		cmd.run(*nuCtx, s)
	}

}

// Call Starts a test tree with an arbitrary HTTP call
func (c *TestCase) Call(description string, method string, path string) *APICmd {
	cmd := &APICmd{narrative: c.narrative + ":" + description, method: method, path: path, headers: map[string]string{}}
	c.tests = append(c.tests, cmd)
	return cmd
}

// StartWithGraph creates a new test tree with an graph
func (c *TestCase) StartWithGraph(msg string) *APICmd {
	return c.Call(msg, http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
}

// Run runs an whole test tree.
func (c *TestCase) Run(t *testing.T, server *server.Server) {
	defer func() {
		// if a run fails keep on going
		for {
			r := recover()
			if r == nil {
				return
			}
			if r != testFailed {
				panic(r)
			}
		}
	}()

	for _, tc := range c.tests {
		ctx := testCtx{t: t}

		tc.run(ctx, server)
	}
}
