package persistence

import (
	"github.com/fnproject/flow/model"
	"io"
)

// BlobStore is an abstraction for user data persistence
// user data is a pure blob with no semantics
// TODO: support streaming in/out of BS using io.Reader/Writer
type BlobStore interface {
	// Read Read a blob from a BlobDatum from the store
	Read(graphID string,datum *model.BlobDatum) (io.Reader, error)

	// Create creates a new blob object associated with a given graph
	Create(graphID string, contentType string, content io.Reader) (*model.BlobDatum, error)

	// Delete removes a blob from the store - this must be idempotent (deleting non-existant blobs must not raise an error)
	Delete(graphID string,datum *model.BlobDatum) error
}
