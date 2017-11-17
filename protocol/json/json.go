// read and write protocol elements to/from JSON
package json

import (
	"io"
	"github.com/fnproject/flow/model"
	"github.com/gogo/protobuf/jsonpb"
)

func ToDatum(reader io.Reader) (*model.Datum, error) {
	datum := &model.Datum{}

	err := jsonpb.Unmarshal(reader, datum)
	if err != nil {
		return nil, err
	}

	return datum, nil
}

func ToCompletionResult(reader io.Reader) (*model.CompletionResult, error) {

}

func ToBlob(reader io.Reader) (*model.BlobDatum, error) {

}

func FromCompletionResult(result *model.CompletionResult, writer io.Writer) error {

}

func FromDatum(result *model.Datum, writer io.Writer) error {

}

func FromBlob(result *model.Datum, writer io.Writer) error {

}


func FromInvokeStageRequest(req * model.InvokeStageRequest, writer io.Writer) error{

}

func FromInvokeFunctionRequest(req * model.InvokeFunctionRequest, writer io.Writer) error{

}