package blobs

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"sync"
)

type inMemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
	count int
}

// NewInMemBlobStore creates an in-mem blob store - use this _only_ for testing - it will eat ur RAMz
func NewInMemBlobStore() Store {
	return &inMemBlobStore{blobs: make(map[string][]byte)}
}

// Read implements BlobStore
func (s *inMemBlobStore) Read(graphID string, blobID string) (io.Reader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs[blobID]

	if !ok {
		return nil, fmt.Errorf("blob %s not found", blobID)
	}

	return bytes.NewReader(blob), nil
}

// Create implements BlobStore
func (s *inMemBlobStore) Create(graphID string, contentType string, data io.Reader) (*Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(s.count)
	s.count++

	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(data)

	if err != nil {
		return nil, err
	}
	s.blobs[id] = buf.Bytes()

	return &Blob{
		ID:          id,
		Length:      uint64(len(s.blobs[id])),
		ContentType: contentType,
	}, nil

}
