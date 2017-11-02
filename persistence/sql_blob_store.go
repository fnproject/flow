package persistence

import (
	"github.com/jmoiron/sqlx"
	"github.com/fnproject/flow/model"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type sqlBlobStore struct {
	snapshotInterval int
	db               *sqlx.DB
}

func NewSqlBlobStore(db *sqlx.DB) (BlobStore, error) {

	log.Info("Creating SQL persistence provider")
	return &sqlBlobStore{
		db: db,
	}, nil
}

func (s *sqlBlobStore) CreateBlob(contentType string, data []byte) (*model.BlobDatum, error) {
	id, err := uuid.NewRandom()

	if err != nil {
		log.WithField("content_type", contentType).WithField("blob_length", len(data)).WithError(err).Errorf("Error generating blob ID")

		return nil, err
	}

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

func (s *sqlBlobStore) ReadBlobData(blob *model.BlobDatum) ([]byte, error) {
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
	return blobData, nil

}
