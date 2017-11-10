package persistence

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path"
)

var tmpDir = path.Clean(os.TempDir())
var dbPath = fmt.Sprintf("%s/flow_test", tmpDir)
var dbFile = fmt.Sprintf("%s/test.db", dbPath)

func testDBURL() *url.URL {
	url, err := url.Parse("sqlite3://" + dbFile)
	if err != nil {
		panic(err)
	}
	return url
}
func resetTestDb() {
	os.RemoveAll(dbPath)
}
