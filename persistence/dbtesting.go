package persistence

import (
	"fmt"
	// Sqlite is required for tests
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path"
)

var tmpDir = path.Clean(os.TempDir())
var dbPath = fmt.Sprintf("%s/flow_test", tmpDir)
var dbFile = fmt.Sprintf("%s/test.db", dbPath)

// TestDBURL is the location of the test DB
func TestDBURL() *url.URL {
	url, err := url.Parse("sqlite3://" + dbFile)
	if err != nil {
		panic(err)
	}
	return url
}

// ResetTestDB clears the test database
func ResetTestDB() {
	os.RemoveAll(dbPath)
}
