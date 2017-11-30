package persistence

import (
	// Sqlite is required for tests
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path"
	"io/ioutil"
)

// TestDBURL is the location of the test DB
func TestDBURL() (*url.URL, string) {
	dbPath, err := ioutil.TempDir("", "flow_test")
	if err != nil {
		panic(err)
	}
	dbFile := path.Clean(path.Join(dbPath, "test.db"))
	url, err := url.Parse("sqlite3://" + dbFile)
	if err != nil {
		panic(err)
	}
	return url, dbPath
}

// ResetTestDB clears the test database
func ResetTestDB(dbPath string) {
	os.RemoveAll(dbPath)
}
