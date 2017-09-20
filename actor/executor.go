package actor

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/protocol"
	"github.com/sirupsen/logrus"
)

const FnCallIDHeader = "Fn_call_id"

type graphExecutor struct {
	faasAddr  string
	client    httpClient
	blobStore persistence.BlobStore
	log       *logrus.Entry
}

// For mocking
type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// ExecHandler abstracts the FaaS execution backend
// implementations must handle all errors and return an appropriate invocation responser
type ExecHandler interface {
	HandleInvokeStage(msg *model.InvokeStageRequest) *model.FaasInvocationResponse
	HandleInvokeFunction(msg *model.InvokeFunctionRequest) *model.FaasInvocationResponse
}

// NewExecutor creates a new executor actor with the given funcions service endpoint
func NewExecutor(faasAddress string, blobStore persistence.BlobStore) actor.Actor {
	client := &http.Client{}
	// TODO configure timeouts
	client.Timeout = 300 * time.Second

	return &graphExecutor{faasAddr: faasAddress,
		log:       logrus.WithField("logger", "executor_actor").WithField("faas_url", faasAddress),
		client:    client,
		blobStore: blobStore,
	}
}

func (exec *graphExecutor) Receive(context actor.Context) {
	sender := context.Sender()
	switch msg := context.Message().(type) {
	case *model.InvokeStageRequest:
		go func() {sender.Tell(exec.HandleInvokeStage(msg))}()
	case *model.InvokeFunctionRequest:
		go func() {sender.Tell(exec.HandleInvokeFunction(msg))}()
	}
}

func (exec *graphExecutor) HandleInvokeStage(msg *model.InvokeStageRequest) *model.FaasInvocationResponse {
	stageLog := exec.log.WithFields(logrus.Fields{"graph_id": msg.GraphId, "stage_id": msg.StageId, "function_id": msg.FunctionId, "operation": msg.Operation})
	stageLog.Info("Running Stage")

	buf := new(bytes.Buffer)

	partWriter := multipart.NewWriter(buf)
	defer partWriter.Close()

	if msg.Closure != nil {
		err := protocol.WritePartFromDatum(exec.blobStore, &model.Datum{Val: &model.Datum_Blob{Blob: msg.Closure}}, partWriter)
		if err != nil {
			exec.log.Error("Failed to create multipart body", err)
			return stageFailed(msg, model.ErrorDatumType_stage_failed, "Error creating stage invoke request", "")

		}
	}
	for _, result := range msg.Args {
		err := protocol.WritePartFromResult(exec.blobStore, result, partWriter)
		if err != nil {
			exec.log.Error("Failed to create multipart body", err)
			return stageFailed(msg, model.ErrorDatumType_stage_failed, "Error creating stage invoke request", "")

		}
	}

	req, _ := http.NewRequest("POST", exec.faasAddr+"/"+msg.FunctionId, buf)
	req.Header.Set("Content-type", fmt.Sprintf("multipart/form-data; boundary=\"%s\"", partWriter.Boundary()))
	req.Header.Set(protocol.HeaderFlowId, msg.GraphId)
	req.Header.Set(protocol.HeaderStageRef, msg.StageId)
	resp, err := exec.client.Do(req)

	if err != nil {
		return stageFailed(msg, model.ErrorDatumType_stage_failed, "HTTP error on stage invocation: Can the completer talk to the functions server?", "")
	}
	defer resp.Body.Close()

	lbDelayHeader := resp.Header.Get("Xxx-Fxlb-Wait")
	if len(lbDelayHeader) > 0 {
		stageLog.WithField("fn_lb_delay", lbDelayHeader).Info("Fn load balancer delay")
	} else {
		stageLog.Info("No Fn load balancer delay header received")
	}

	callId := resp.Header.Get(FnCallIDHeader)

	if !successfulResponse(resp) {
		stageLog.WithField("http_status", fmt.Sprintf("%d", resp.StatusCode)).Error("Got non-200 error from FaaS endpoint")

		if resp.StatusCode == 504 {
			return &model.FaasInvocationResponse{GraphId: msg.GraphId, StageId: msg.StageId, FunctionId: msg.FunctionId, Result: model.NewInternalErrorResult(model.ErrorDatumType_stage_timeout, "stage timed out"), CallId: callId}
		}
		return stageFailed(msg, model.ErrorDatumType_stage_failed, fmt.Sprintf("Invalid http response from functions platform code %d", resp.StatusCode), callId)
	}

	result, err := protocol.CompletionResultFromEncapsulatedResponse(exec.blobStore, resp)
	if err != nil {
		stageLog.Error("Failed to read result from functions service", err)
		return stageFailed(msg, model.ErrorDatumType_invalid_stage_response, "Failed to read result from functions service", callId)

	}
	stageLog.WithField("successful", fmt.Sprintf("%t", result.Successful)).Info("Got stage response")

	return &model.FaasInvocationResponse{GraphId: msg.GraphId, StageId: msg.StageId, FunctionId: msg.FunctionId, Result: result, CallId: callId}
}

func stageFailed(msg *model.InvokeStageRequest, errorType model.ErrorDatumType, errorMessage string, callId string) *model.FaasInvocationResponse {
	return &model.FaasInvocationResponse{GraphId: msg.GraphId, StageId: msg.StageId, FunctionId: msg.FunctionId, Result: model.NewInternalErrorResult(errorType, errorMessage), CallId: callId}
}


func (exec *graphExecutor) HandleInvokeFunction(msg *model.InvokeFunctionRequest) *model.FaasInvocationResponse {
	datum := msg.Arg

	method := strings.ToUpper(model.HttpMethod_name[int32(datum.Method)])
	stageLog := exec.log.WithFields(logrus.Fields{"graph_id": msg.GraphId, "stage_id": msg.StageId, "target_function_id": msg.FunctionId, "method": method})
	stageLog.Info("Sending function invocation")

	var bodyReader io.Reader

	if datum.Body != nil {
		blobData, err := exec.blobStore.ReadBlobData(datum.Body)
		if err != nil {
			return invokeFailed(msg, "Failed to read data for invocation", "")
		}
		bodyReader = bytes.NewReader(blobData)
	} else {
		bodyReader = http.NoBody
	}

	req, err := http.NewRequest(strings.ToUpper(method), exec.faasAddr+"/"+msg.FunctionId, bodyReader)
	if err != nil {
		exec.log.Error("Failed to create http request:", err)
		return invokeFailed(msg, "Failed to create HTTP request", "")
	}

	if datum.Body != nil {
		req.Header.Set("Content-Type", datum.Body.ContentType)
	}

	resp, err := exec.client.Do(req)

	if err != nil {
		exec.log.Error("Http error calling functions service:", err)
		return invokeFailed(msg, "Failed to call function", "")

	}
	defer resp.Body.Close()

	lbDelayHeader := resp.Header.Get("Xxx-Fxlb-Wait")
	if len(lbDelayHeader) > 0 {
		stageLog.WithField("fn_lb_delay", lbDelayHeader).Info("Fn load balancer delay")
	} else {
		stageLog.Info("No Fn load balancer delay header received")
	}

	callId := resp.Header.Get(FnCallIDHeader)
	buf := &bytes.Buffer{}
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		exec.log.Error("Error reading data from function:", err)
		return invokeFailed(msg, "Failed to call function could not read response", callId)

	}

	var contentType = resp.Header.Get("Content-type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var headers = make([]*model.HttpHeader, 0)
	for headerName, valList := range resp.Header {
		// Don't copy content type into headers
		if textproto.CanonicalMIMEHeaderKey(headerName) == "Content-Type" {
			continue
		}
		for _, val := range valList {
			headers = append(headers, &model.HttpHeader{Key: headerName, Value: val})
		}
	}

	bytes := buf.Bytes()
	blob, err := exec.blobStore.CreateBlob(contentType, bytes)
	if err != nil {
		return invokeFailed(msg, "Failed to persist HTTP response data", callId)
	}

	resultDatum := &model.Datum{
		Val: &model.Datum_HttpResp{
			HttpResp: &model.HttpRespDatum{
				Headers:    headers,
				Body:       blob,
				StatusCode: uint32(resp.StatusCode)}}}

	result := &model.CompletionResult{Successful: successfulResponse(resp), Datum: resultDatum}
	return &model.FaasInvocationResponse{GraphId: msg.GraphId, StageId: msg.StageId, FunctionId: msg.FunctionId, Result: result, CallId: callId}
}

func successfulResponse(resp *http.Response) bool {
	// assume any non-error codes are successful
	// TODO doc in spec
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func invokeFailed(msg *model.InvokeFunctionRequest, errorMessage string, callId string) *model.FaasInvocationResponse {
	return &model.FaasInvocationResponse{GraphId: msg.GraphId, StageId: msg.StageId, FunctionId: msg.FunctionId, Result: model.NewInternalErrorResult(model.ErrorDatumType_function_invoke_failed, errorMessage), CallId: callId}
}
