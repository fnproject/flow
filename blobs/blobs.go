package blobs

import (
	"io"
)

type Blob struct {
	ID          string `json:"blob_id,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Length      uint64 `json:"length,omitempty"`
}

// Store is an abstraction for user data persistence
// user data is a pure blob with no semantics
// TODO: support streaming in/out of BS using io.Reader/Writer
type Store interface {
	// Read Read a blob from a BlobDatum from the store
	Read(prefix string, blobID string) (io.Reader, error)

	// Create creates a new blob object associated with a given graph
	Create(prefix string, contentType string, content io.Reader) (*Blob, error)

}
