package persistence

import (
	"net/url"
	"github.com/jmoiron/sqlx"
	"fmt"
	"path/filepath"
	"os"
	"strings"
	"github.com/sirupsen/logrus"
	"database/sql"
)

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

	`CREATE TABLE IF NOT EXISTS blobs (
	 blob_id varchar(255) NOT NULL PRIMARY KEY ,
	 blob_data BLOB);`,
}

// CreateDBConnecection sets up a DB connection and ensures required tables exist
func CreateDBConnecection(url *url.URL) (*sqlx.DB, error) {
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

	return sqlxDb, nil

}
