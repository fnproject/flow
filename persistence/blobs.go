package persistence

import (
	"fmt"
	"sync"
	"strconv"
	"github.com/fnproject/completer/model"
)

type BlobStore interface {
	ReadBlobData(datum *model.BlobDatum) ([]byte, error)
	CreateBlob(contentType string, content []byte) (*model.BlobDatum, error)
}

type inMemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
	count int
}

func NewInMemBlobStore() BlobStore {
	return &inMemBlobStore{blobs: make(map[string][]byte)}
}

func (s *inMemBlobStore) ReadBlobData(datum *model.BlobDatum) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs[datum.BlobId]

	if !ok {
		return nil, fmt.Errorf("Blob %s not found", datum.BlobId)
	}
	return blob, nil
}

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
