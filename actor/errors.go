package actor

import "github.com/fnproject/completer/model"

func NewInvalidGraphOperation(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph not found"}
}
