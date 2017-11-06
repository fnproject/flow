package persistence

import (
	"github.com/golang/protobuf/proto"
	"fmt"
	"github.com/sirupsen/logrus"
	"database/sql"
	"github.com/jmoiron/sqlx"
	"reflect"
	"github.com/opentracing/opentracing-go"
)

type SqlProvider struct {
	snapshotInterval int
	db               *sqlx.DB
}

var log = logrus.New().WithField("logger", "persistence")

func NewSqlProvider(db *sqlx.DB, snapshotInterval int) (ProviderState, error) {

	log.Info("Creating SQL persistence provider")
	return &SqlProvider{
		snapshotInterval: snapshotInterval,
		db:               db,
	}, nil
}

func (provider *SqlProvider) Restart() {}

func (provider *SqlProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *SqlProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {

	row := provider.db.QueryRowx("SELECT snapshot_type,event_index,snapshot FROM snapshots WHERE actor_name = ?", actorName)

	if row.Err() != nil {
		log.WithField("actor_name", actorName).Error("Error getting snapshot value from DB ", row.Err())
		return nil, -1, false
	}

	var snapshotType string
	var snapshotBytes []byte

	err := row.Scan(&snapshotType, &eventIndex, &snapshotBytes)
	if err == sql.ErrNoRows {
		return nil, -1, false
	}

	if err != nil {
		log.WithField("actor_name", actorName).Error("Error snapshot value from DB ", err)
		return nil, -1, false
	}
	message, err := extractData(actorName, snapshotType, snapshotBytes)

	if err != nil {
		log.WithFields(logrus.Fields{"actor_name": actorName, "message_type": snapshotType}).WithError(err).Errorf("Failed to read  protobuf for snapshot")
		return nil, -1, false
	}

	return message, eventIndex, true
}

func extractData(actorName string, msgTypeName string, msgBytes []byte) (proto.Message, error) {
	protoType := proto.MessageType(msgTypeName)

	if protoType == nil {
		log.WithFields(logrus.Fields{"actor_name": actorName, "message_type": msgTypeName}).Errorf("protocol type not supported by protobuf")
		return nil, fmt.Errorf("Unsupported protocol type %s", protoType)
	}
	t := protoType.Elem()
	intPtr := reflect.New(t)
	message := intPtr.Interface().(proto.Message)

	err := proto.Unmarshal(msgBytes, message)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (provider *SqlProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	pbType := proto.MessageName(snapshot)
	pbBytes, err := proto.Marshal(snapshot)

	if err != nil {
		panic(err)
	}

	_, err = provider.db.Exec("REPLACE INTO snapshots (actor_name,snapshot_type,event_index,snapshot) VALUES (?,?,?,?)",
		actorName, pbType, eventIndex, pbBytes)

	if err != nil {
		panic(err)
	}
}

func (provider *SqlProvider) GetEvents(actorName string, eventIndexStart int, callback func(eventIndex int, e interface{})) {
	log.WithFields(logrus.Fields{"actor_name":actorName,"event_index":eventIndexStart}).Debug("Getting events")
	span := opentracing.StartSpan("sql_get_events")
	defer span.Finish()
	rows, err := provider.db.Queryx("SELECT event_type,event_index,event FROM events where actor_name = ? AND event_index >= ? ORDER BY event_index ASC", actorName, eventIndexStart)
	if err != nil {
		log.WithField("actor_name", actorName).WithError(err).Error("Error getting events value from DB ")

		// DON't PANIC ?
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventType string
		var eventIndex int
		var eventBytes []byte
		rows.Scan(&eventType, &eventIndex, &eventBytes)

		msg, err := extractData(actorName, eventType, eventBytes)
		if err != nil {
			panic(err)
		}
		callback(eventIndex,msg)
	}

}

func (provider *SqlProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {

	log.WithFields(logrus.Fields{"actor_name":actorName,"event_index":eventIndex}).Debug("Persisting event")

	pbType := proto.MessageName(event)
	pbBytes, err := proto.Marshal(event)

	if err != nil {
		panic(err)
	}

	span := opentracing.StartSpan("sql_persist_event")
	defer span.Finish()
	_, err = provider.db.Exec("REPLACE INTO events (actor_name,event_type,event_index,event) VALUES (?,?,?,?)",
		actorName, pbType, eventIndex, pbBytes)
	if err != nil {
		panic(err)
	}
}
