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
)

type SqlProvider struct {
	snapshotInterval int
	db               *sqlx.DB
}

var tables = [...]string{`CREATE TABLE IF NOT EXISTS events (
	actor_name varchar(255) NOT NULL,
	message_type varchar(255) NOT NULL,
	message_index int NOT NULL,
	message BLOB NOT NULL);`,

	`CREATE TABLE IF NOT EXISTS snapshots (
	actor_name varchar(255) NOT NULL PRIMARY KEY ,
	message_type varchar(255) NOT NULL,
	message_index int NOT NULL,
	snapshot BLOB NOT NULL);`,
}

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

	result, err := provider.db.Query("SELECT message_type,message_index,message FROM snapshots WHERE actor_name = ?", actorName)

	if err != nil {
		return nil, -1, false;
	}
	defer result.Close()

	if !result.Next() {
		return nil,-1,false
	}
	result.Columns()



	return nil, 0, false
}

func (provider *SqlProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	pbType := proto.MessageName(snapshot)
	pbBytes, err := proto.Marshal(snapshot)

	if err != nil {
		panic(err)
	}

	_, err = provider.db.Exec("INSERT OR REPLACE INTO snapshots (actor_name,message_type,message_index,message) VALUES (?,?,?,?)",
		actorName, pbType, eventIndex, pbBytes)

	if err != nil {
		panic(err)
	}
}

func (provider *SqlProvider) GetEvents(actorName string, eventIndexStart int, callback func(e interface{})) {

}

func (provider *SqlProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {

}
