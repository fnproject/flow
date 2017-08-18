package persistence

import (
	"github.com/golang/protobuf/proto"
	"net/url"
	"fmt"
	"path/filepath"
	"os"
	"github.com/sirupsen/logrus"
	"database/sql"
	"github.com/jmoiron/sqlx"
	"strings"
	"reflect"
)

type SqlProvider struct {
	snapshotInterval int
	db               *sqlx.DB
}

var tables = [...]string{`CREATE TABLE IF NOT EXISTS events (
	actor_name varchar(255) NOT NULL,
	event_type varchar(255) NOT NULL,
	event_index int NOT NULL,
	event BLOB NOT NULL);`,

	`CREATE TABLE IF NOT EXISTS snapshots (
	actor_name varchar(255) NOT NULL PRIMARY KEY ,
	snapshot_type varchar(255) NOT NULL,
	event_index int NOT NULL,
	snapshot BLOB NOT NULL);`,
}

var log = logrus.New().WithField("logger", "sql_persistence")

func NewSqlProvider(url *url.URL, snapshotInterval int) (*SqlProvider, error) {

	driver := url.Scheme
	switch driver {
	case "mysql", "sqlite3":
	default:

		return nil, fmt.Errorf("Invalid db driver %s", driver)
	}

	if driver == "sqlite3" {
		dir := filepath.Dir(url.Path)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}
	var uri = url.String()

	uri = strings.TrimPrefix(url.String(), url.Scheme+"://")

	sqldb, err := sql.Open(driver, uri)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't open db")
		return nil, err
	}

	sqlxDb := sqlx.NewDb(sqldb, driver)
	err = sqlxDb.Ping()
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't ping db")
		return nil, err
	}

	maxIdleConns := 256 // TODO we need to strip this out of the URL probably
	sqlxDb.SetMaxIdleConns(maxIdleConns)
	switch driver {
	case "sqlite3":
		sqlxDb.SetMaxOpenConns(1)
	}
	for _, v := range tables {
		_, err = sqlxDb.Exec(v)
		if err != nil {
			return nil, fmt.Errorf("Failed to create database table %s: %v", v, err)
		}
	}
	return &SqlProvider{
		snapshotInterval: snapshotInterval,
		db:               sqlxDb,
	}, nil
}

func (provider *SqlProvider) Restart() {}

func (provider *SqlProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *SqlProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {

	row := provider.db.QueryRowx("SELECT snapshot_type,event_index,snapshot FROM snapshots WHERE actor_name = ?", actorName)

	if row.Err() != nil {
		log.WithField("actor_name", actorName).Errorf("Error getting snapshot value from DB ", row.Err())
		return nil, -1, false
	}

	var snapshotType string
	var snapshotBytes []byte

	err := row.Scan(&snapshotType, &eventIndex, &snapshotBytes)
	if err == sql.ErrNoRows {
		return nil, -1, false
	}

	if err != nil {
		log.WithField("actor_name", actorName).Errorf("Error snapshot value from DB ", err)
		return nil, -1, false
	}

	protoType := proto.MessageType(snapshotType)

	if protoType == nil {
		log.WithFields(logrus.Fields{"actor_name": actorName, "message_type": snapshotType}).Errorf("snapshot type not supported by protobuf")
		return nil, -1, false
	}
	t := protoType.Elem()
	intPtr := reflect.New(t)
	message := intPtr.Interface().(proto.Message)

	err = proto.Unmarshal(snapshotBytes, message.(proto.Message))

	if err != nil {
		log.WithFields(logrus.Fields{"actor_name": actorName, "message_type": snapshotType}).WithError(err).Errorf("Failed to read  protobuf for snapshot")
		return nil, -1, false
	}

	return message, eventIndex, true
}

func (provider *SqlProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	pbType := proto.MessageName(snapshot)
	pbBytes, err := proto.Marshal(snapshot)

	if err != nil {
		panic(err)
	}

	_, err = provider.db.Exec("INSERT OR REPLACE INTO snapshots (actor_name,snapshot_type,event_index,snapshot) VALUES (?,?,?,?)",
		actorName, pbType, eventIndex, pbBytes)

	if err != nil {
		panic(err)
	}
}

func (provider *SqlProvider) GetEvents(actorName string, eventIndexStart int, callback func(e interface{})) {
	row := provider.db.QueryRowx("SELECT event_type,event_index,event FROM events where actor_name = ? AND event_index >= ?", actorName, eventIndexStart)

	if row.Err() != nil {
		log.WithField("actor_name", actorName).WithError(row.Err()).Error("Error getting events value from DB ")

		// DON't PANIC ?
		panic(row.Err())
	}

}

func (provider *SqlProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {

}
