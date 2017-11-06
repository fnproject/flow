package persistence

import (
	"fmt"
	"sync"
	"strconv"
	"github.com/fnproject/flow/model"
)

// BlobStore is an abstraction for user data persistence
// user data is a pure blob with no semantics
// TODO: support streaming in/out of BS using io.Reader/Writer
type BlobStore interface {
	// ReadBlobData Read a blob from a BlobDatum from the store
	ReadBlobData(datum *model.BlobDatum) ([]byte, error)

	// CreateBlob creates a new blob object
	CreateBlob(contentType string, content []byte) (*model.BlobDatum, error)
}

type inMemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
	count int
}

// NewInMemBlobStore creates an in-mem blob store - use this _only_ for testing - it will eat ur RAMz
func NewInMemBlobStore() BlobStore {
	return &inMemBlobStore{blobs: make(map[string][]byte)}
}

// ReadBlobData extracts  the in-mem blob data
func (s *inMemBlobStore) ReadBlobData(datum *model.BlobDatum) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs[datum.BlobId]

	if !ok {
		return nil, fmt.Errorf("blob %s not found", datum.BlobId)
	}
	return blob, nil
}

// CreateBlob puts the blob in memory
func (s *inMemBlobStore) CreateBlob(contentType string, byte []byte) (*model.BlobDatum, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(s.count)
	s.count++
	s.blobs[id] = byte

	return &model.BlobDatum{
		BlobId:      id,
		Length:      uint64(len(byte)),
		ContentType: contentType,
	}, nil

}
