package persistence

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	// this is pulled in to ensure we have mysql Drivers for DB/DBX
	_ "github.com/go-sql-driver/mysql"
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
	 graph_id varchar(255) NOT NULL,
	 blob_id varchar(255) NOT NULL PRIMARY KEY ,
	 blob_data BLOB);`,
}

// CreateDBConnection sets up a DB connection and ensures required tables exist
func CreateDBConnection(url *url.URL) (*sqlx.DB, error) {
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

	maxIdleConns := 10 // TODO we need to strip this out of the URL probably
	switch driver {
	case "sqlite3":
		sqlxDb.SetMaxIdleConns(1)
		sqlxDb.SetMaxOpenConns(1)
	case "mysql":
		sqlxDb.SetMaxIdleConns(maxIdleConns)
		sqlxDb.SetMaxOpenConns(5 * maxIdleConns)
		// setting the lifetime seems to result in driver bad connection errors
		// sqlxDb.SetConnMaxLifetime(1 * time.Minute)
	}
	for _, v := range tables {
		_, err = sqlxDb.Exec(v)
		if err != nil {
			return nil, fmt.Errorf("Failed to create database table %s: %v", v, err)
		}
	}

	return sqlxDb, nil

}
