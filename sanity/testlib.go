package sanity

import (
	"bytes"
	"fmt"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type testCtx struct {
	failed       bool
	narrative    []string
	t            *testing.T
	graphId      string
	stageId      string
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

type testCase struct {
	narrative string
	tests     []*apiCmd
}

func NewCase(narrative string) *testCase {
	return &testCase{narrative: narrative}
}

type resultFunc func(ctx *testCtx, response *http.Response)

type resultAction struct {
	narrative string
	action    resultFunc
}

// operation on root API
type apiCmd struct {
	narrative string
	path      string
	method    string
	headers   map[string]string
	body      []byte
	expect    []*resultAction
	cmd       []*apiCmd
}

func (tc *testCtx) Errorf(msg string, args ...interface{}) {
	tc.failed = true
	tc.t.Errorf("Expectation failed: \n\t\t%s ", strings.Join(tc.narrative, "\n\t\t ->  "))
	tc.t.Errorf(msg, args...)
	tc.t.Error("Last Response was: %v", tc.lastResponse)
}

const testFailed = "TestFailed"

func (tc *testCtx) FailNow() {
	panic(testFailed)
}

func (c *apiCmd) WithHeader(k string, v string) *apiCmd {
	c.headers[k] = v
	return c
}

func (c *apiCmd) WithHeaders(h map[string]string) *apiCmd {
	for k, v := range h {
		c.headers[k] = v
	}
	return c
}

func (c *apiCmd) Expect(fn resultFunc, msg string, args ...interface{}) *apiCmd {
	c.expect = append(c.expect, &resultAction{fmt.Sprintf(msg, args...), fn})
	return c
}

func (c *apiCmd) ExpectStatus(status int) *apiCmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, status, resp.StatusCode, "Http status should be %d", status)
	}, "status matches %d", status)
}

func (c *apiCmd) ExpectGraphCreated() *apiCmd {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *http.Response) {
			flowIdHeader := resp.Header.Get("Fnproject-FlowId")
			require.NotEmpty(ctx, flowIdHeader, "FlowId header must be present in headers %v ", resp.Header)
			ctx.graphId = flowIdHeader
		}, "Graph was created")
}

func (c *apiCmd) ExpectStageCreated() *apiCmd {
	return c.ExpectStatus(200).
		Expect(func(ctx *testCtx, resp *http.Response) {
			stage := resp.Header.Get("Fnproject-StageId")
			require.NotEmpty(ctx, stage, "StageID not in header")
			ctx.stageId = stage
		}, "Stage was created")
}

func (c *apiCmd) ExpectLastStageEvent(test func(*testCtx, *model.StageAddedEvent)) *apiCmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		var lastStageAddedEvent *model.StageAddedEvent

		ctx.server.GraphManager.QueryGraphEvents(ctx.graphId, 0,
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

func (c *apiCmd) ExpectRequestErr(serverErr error) *apiCmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, 400, resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.Header.Get("content-type"))
		buf := &bytes.Buffer{}
		_, err := buf.ReadFrom(resp.Body)
		require.NoError(ctx, err)
		assert.Equal(ctx, serverErr.Error(), string(buf.Bytes()), "Error body did not match")
	}, "Request Error :  %s", serverErr.Error())
}

func (c *apiCmd) ExpectServerErr(serverErr *server.ServerErr) *apiCmd {
	return c.Expect(func(ctx *testCtx, resp *http.Response) {
		assert.Equal(ctx, serverErr.HttpStatus, resp.StatusCode)
		assert.Equal(ctx, "text/plain", resp.Header.Get("content-type"))
		buf := &bytes.Buffer{}
		_, err := buf.ReadFrom(resp.Body)
		require.NoError(ctx, err)
		assert.Equal(ctx, serverErr.Message, string(buf.Bytes()), "Error body did not match")
	}, "Server Error : %d %s", serverErr.HttpStatus, serverErr.Message)
}

func (c *apiCmd) WithBodyString(data string) *apiCmd {
	c.body = []byte(data)
	return c

}

func (c *apiCmd) WithBlobDatum(contentType string, data string) *apiCmd {
	return c.WithBodyString(data).WithHeaders(map[string]string{
		"content-type":        contentType,
		"fnproject-datumtype": "blob",
	})

}

func (c *apiCmd) WithErrorDatum(errorType string, message string) *apiCmd {
	return c.WithBodyString(message).WithHeaders(map[string]string{
		"content-type":        "text/plain",
		"fnproject-errortype": errorType,
		"fnproject-datumtype": "error",
	})
}

func (c *apiCmd) ThenCall(method string, path string) *apiCmd {
	newCmd := &apiCmd{method: method, path: path, headers: map[string]string{}}
	c.cmd = append(c.cmd, newCmd)
	return newCmd
}

func (c *apiCmd) ToReq(ctx *testCtx) *http.Request {
	placeholders := func(key string) string {

		var s = strings.Replace(key, ":graphId", ctx.graphId, -1)
		s = strings.Replace(s, ":stageId", ctx.stageId, -1)
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

func (c *apiCmd) Run(ctx testCtx, s *server.Server) {
	ctx.server = s
	nuctx := (&ctx).PushNarrative(c.narrative)

	req := c.ToReq(nuctx)
	resp := httptest.NewRecorder()

	nuctx = nuctx.PushNarrative(fmt.Sprintf("%s %s", req.Method, req.URL))
	fmt.Printf("Test : %s\n", nuctx)

	s.Engine.ServeHTTP(resp, req)
	nuctx.lastResponse = resp.Result()

	for _, check := range c.expect {
		nuctx = nuctx.PushNarrative(check.narrative)
		check.action(nuctx, nuctx.lastResponse)
	}

	for _, cmd := range c.cmd {
		cmd.Run(*nuctx, s)
	}

}

func (c *testCase) Call(description string, method string, path string) *apiCmd {
	cmd := &apiCmd{narrative: c.narrative + ":" + description, method: method, path: path, headers: map[string]string{}}
	c.tests = append(c.tests, cmd)
	return cmd
}

func (c *testCase) StartWithGraph(msg string) *apiCmd {
	return c.Call(msg, http.MethodPost, "/graph?functionId=testapp/fn").ExpectGraphCreated()
}

func (tc *testCase) Run(t *testing.T, server *server.Server) {

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

	for _, tc := range tc.tests {
		ctx := testCtx{t: t}

		tc.Run(ctx, server)
	}
}
