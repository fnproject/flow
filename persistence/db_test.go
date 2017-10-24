package persistence

import (
	"path"
	"os"
	"fmt"
	"net/url"
	_ "github.com/mattn/go-sqlite3"

)

var tmpDir = path.Clean(os.TempDir())
var dbPath = fmt.Sprintf("%s/flow_test", tmpDir)
var dbFile = fmt.Sprintf("%s/test.db", dbPath)


func testDbUrl() *url.URL {
	url, err := url.Parse("sqlite3://" + dbFile)
	if err != nil {
		panic(err)
	}
	return url
}
func resetTestDb() {
	os.RemoveAll(dbPath)
}

