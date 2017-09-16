package model

import (
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// This contains mixins that add operations and types to the protobuf messages

// GetHeaderValues returns a list of values of the headers with the corresponding key in HttpReqDatum
func (m *HttpReqDatum) GetHeaderValues(key string) []string {
	res := make([]string, 0)
	if m != nil {
		for _, h := range m.Headers {
			if h.Key == key {
				res = append(res, h.Value)
			}
		}
	}
	return res
}

// GetHeader returns the first header with the corresponding key in the HttpReqDatum, or an empty string if not found
func (m *HttpReqDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// GetHeaderValues returns a list of values of the headers with the corresponding key in HttpRespDatum
func (m *HttpRespDatum) GetHeaderValues(key string) []string {
	res := make([]string, 0)
	if m != nil {
		for _, h := range m.Headers {
			if h.Key == key {
				res = append(res, h.Value)
			}
		}
	}
	return res
}

// Returns the first header with the corresponding key in the HttpRespDatum, or an empty string if not found
func (m *HttpRespDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// Turn some messages into errors - these will be directed as errors on the API

func (m *InvalidStageOperation) Error() string {
	return m.Err
}

func (m *InvalidGraphOperation) Error() string {
	return m.Err
}

type GraphMessage interface {
	proto.Message
	GetGraphId() string
}

type StageMessage interface {
	proto.Message
	GetGraphId() string
	GetStageId() string
}

// Event is the base interface for all things that may be persisted to the Journal
type Event interface {
	proto.Message
	GetTs() *timestamp.Timestamp
}

// Command is the base interface for all user-facing graph requests
type Command interface {
	GraphMessage
}

// AddStageCommand is any command that creates a stage  and Warrants an AddStageResponse
type AddStageCommand interface {
	GetGraphId() string
	GetOperation() CompletionOperation
	GetDependencyCount() int
	GetCodeLocation() string
}

func (m *AddExternalCompletionStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_externalCompletion
}

func (m *AddExternalCompletionStageRequest) GetDependencyCount() int {
	return 0
}

func (m *AddCompletedValueStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_completedValue
}

func (m *AddCompletedValueStageRequest) GetDependencyCount() int {
	return 0
}

func (m *AddDelayStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_delay
}

func (m *AddInvokeFunctionStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_invokeFunction
}

func (m *AddChainedStageRequest) GetDependencyCount() int {
	return len(m.Deps)
}



func (m *AddDelayStageRequest) GetDependencyCount() int {
	return 0
}

func (m *AddInvokeFunctionStageRequest) GetDependencyCount() int {
	return 0
}
