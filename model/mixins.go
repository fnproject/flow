package model

import (
	"github.com/fnproject/flow/blobs"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// This contains mixins that add operations and types to the protobuf messages

// GetHeaderValues returns a list of values of the headers with the corresponding key in HttpReqDatum
func (m *HTTPReqDatum) GetHeaderValues(key string) []string {
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
func (m *HTTPReqDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// GetHeaderValues returns a list of values of the headers with the corresponding key in HttpRespDatum
func (m *HTTPRespDatum) GetHeaderValues(key string) []string {
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

// GetHeader returns the first header with the corresponding key in the HttpRespDatum, or an empty string if not found
func (m *HTTPRespDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (m *InvalidStageOperation) Error() string {
	return m.Err
}

func (m *InvalidGraphOperation) Error() string {
	return m.Err
}

// GraphMessage is any message that belongs exclusively to a graph
type GraphMessage interface {
	proto.Message
	GetGraphId() string
	SetGraphId(string)

}

// StageMessage is any message that belongs exclusively a stage (and hence a graph)
// This is intentionally distinct from GraphMessage!
type StageMessage interface {
	proto.Message
	GetGraphId() string
	SetGraphId(string)
	GetStageId() string
	SetStageId(string)
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
	GetCallerId() string
}

// GetOperation for AddStageCommand.GetOperation
func (m *AddExternalCompletionStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_externalCompletion
}

// GetDependencyCount for AddStageCommand.GetDependencyCount
func (m *AddExternalCompletionStageRequest) GetDependencyCount() int {
	return 0
}

// GetOperation for AddStageCommand.GetOperation
func (m *AddCompletedValueStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_completedValue
}

// GetDependencyCount for AddStageCommand.GetDependencyCount
func (m *AddCompletedValueStageRequest) GetDependencyCount() int {
	return 0
}

// GetOperation for AddStageCommand.GetOperation
func (m *AddDelayStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_delay
}

// GetOperation for AddStageCommand.GetOperation
func (m *AddInvokeFunctionStageRequest) GetOperation() CompletionOperation {
	return CompletionOperation_invokeFunction
}

// GetDependencyCount for AddStageCommand.GetDependencyCount
func (m *AddChainedStageRequest) GetDependencyCount() int {
	return len(m.Deps)
}

// GetDependencyCount for AddStageCommand.GetDependencyCount
func (m *AddDelayStageRequest) GetDependencyCount() int {
	return 0
}

// GetDependencyCount for AddStageCommand.GetDependencyCount
func (m *AddInvokeFunctionStageRequest) GetDependencyCount() int {
	return 0
}

// BlobDatumFromBlobStoreBlob creates a model blob from a blobstore result
func BlobDatumFromBlobStoreBlob(b *blobs.Blob) *BlobDatum {
	return &BlobDatum{
		BlobId:      b.ID,
		ContentType: b.ContentType,
		Length:      b.Length,
	}
}
