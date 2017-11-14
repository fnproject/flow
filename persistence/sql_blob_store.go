package persistence

import (
	"github.com/fnproject/flow/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opentracing/opentracing-go"
	"io"
	"bytes"
)

type sqlBlobStore struct {
	snapshotInterval int
	db               *sqlx.DB
}


// NewSQLBlobStore creates a new blob store on the given DB , the DB should already have tables in place
func NewSQLBlobStore(db *sqlx.DB) (BlobStore, error) {

	log.Info("Creating SQL persistence provider")
	return &sqlBlobStore{
		db: db,
	}, nil
}


// Create implements BlobStore - this buffers the blob to send to the DB
func (s *sqlBlobStore) Create(graphID string, contentType string, input io.Reader) (*model.BlobDatum, error) {
	id, err := uuid.NewRandom()

	if err != nil {
		log.WithField("content_type", contentType).WithError(err).Errorf("Error generating blob ID")
		return nil, err
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(input)

	if err != nil {
		return nil, err
	}
	data := buf.Bytes()

	idString := id.String()

	span := opentracing.StartSpan("sql_create_blob")
	defer span.Finish()
	_, err = s.db.Exec("INSERT INTO blobs(blob_id,blob_data) VALUES(?,?)", idString, data)
	if err != nil {
		log.WithField("content_type", contentType).WithField("blob_length", len(data)).WithError(err).Errorf("Error inserting blob into db")
		return nil, err
	}

	return &model.BlobDatum{
		BlobId:      idString,
		Length:      uint64(len(data)),
		ContentType: contentType,
	}, nil
}

// Read implements BlobStore - this buffers the blob before creating a reader
func (s *sqlBlobStore) Read(graphID string, blob *model.BlobDatum) (io.Reader, error) {
	span := opentracing.StartSpan("sql_read_blob_data")
	defer span.Finish()
	row := s.db.QueryRowx("SELECT blob_data FROM blobs where blob_id = ?", blob.BlobId)
	if row.Err() != nil {
		log.WithField("blob_id", blob.BlobId).WithError(row.Err()).Errorf("Error querying blob from DB ")
		return nil, row.Err()
	}

	var blobData []byte
	err := row.Scan(&blobData)

	if err != nil {

		log.WithField("blob_id", blob.BlobId).WithError(row.Err()).Errorf("Error reading blob from DB")
		return nil, err
	}
	return bytes.NewBuffer(blobData), nil

}

// Delete implements BlobStore
func (s *sqlBlobStore) Delete(graphID string, blob *model.BlobDatum) error {
	span := opentracing.StartSpan("sql_delete_blob")


	defer span.Finish()

	return nil
}
