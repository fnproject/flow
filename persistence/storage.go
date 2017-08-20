package persistence

import (
	"github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/fnproject/completer/model"
)

type Storage interface {
	persistence.ProviderState
	model.BlobStore
}

