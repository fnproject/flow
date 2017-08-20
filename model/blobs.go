package model

import (
	"fmt"
	"sync"
	"strconv"
)

type BlobStore interface {
	ReadBlob(datum *BlobDatum) ([]byte, error)
	CreateBlob(contentType string, content []byte) (*BlobDatum, error)
}

type inMemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
	count int
}

func NewInMemBlobStore() BlobStore {
	return &inMemBlobStore{blobs: make(map[string][]byte)}
}

func (s *inMemBlobStore) ReadBlob(datum *BlobDatum) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs[datum.BlobId]

	if !ok {
		return nil, fmt.Errorf("Blob %s not found", datum.BlobId)
	}
	return blob, nil
}

func (s *inMemBlobStore) CreateBlob(contentType string, byte []byte) (*BlobDatum, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(s.count)
	s.count++
	s.blobs[id] = byte

	return &BlobDatum{
		BlobId:      id,
		Length:      uint64(len(byte)),
		ContentType: contentType,
	}, nil

}
