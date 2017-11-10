package persistence

import (
	"sync"
	"fmt"
	"bytes"
	"github.com/fnproject/flow/model"
	"io"
	"strconv"
)

type inMemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
	count int
}

// NewInMemBlobStore creates an in-mem blob store - use this _only_ for testing - it will eat ur RAMz
func NewInMemBlobStore() BlobStore {
	return &inMemBlobStore{blobs: make(map[string][]byte)}
}

// Read implements BlobStore
func (s *inMemBlobStore) Read(graphID string,datum *model.BlobDatum) (io.Reader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs[graphID + "-" + datum.BlobId]

	if !ok {
		return nil, fmt.Errorf("blob %s not found", datum.BlobId)
	}

	return bytes.NewReader(blob), nil
}

// Create implements BlobStore
func (s *inMemBlobStore) Create(graphID string,contentType string, data io.Reader) (*model.BlobDatum, error) {
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

	return &model.BlobDatum{
		BlobId:      id,
		Length:      uint64(len(s.blobs[id])),
		ContentType: contentType,
	}, nil

}

// Delete implements BlobStore
func (s *inMemBlobStore) Delete(graphID string , datum *model.BlobDatum) error {
	delete(s.blobs, datum.BlobId)
	return nil
}

