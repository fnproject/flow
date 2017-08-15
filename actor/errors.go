package actor

import "github.com/fnproject/completer/model"

func NewGraphNotFoundError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph not found"}
}

func NewGraphEventPersistenceError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Failed to persist event"}
}
