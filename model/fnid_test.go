package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestFunctionIdValidation(t *testing.T) {

	longestAcceptableAppID := strings.Repeat("v", 255)
	cases := []struct {
		fnID  string
		works bool
		appID string
		path  string
		query string
	}{
		{"", false, "", "", ""},
		{" ", false, "", "", ""},
		{" s", false, "", "", ""},
		{"s ", false, "", "", ""},
		{"&&&", false, "", "", ""},
		{"app/", true, "app", "/", ""},
		{"Myapp/", true, "Myapp", "/", ""},
		{"myapp/myfn", true, "myapp", "/myfn", ""},
		{"./myfn", true, ".", "/myfn", ""},
		{longestAcceptableAppID + "/path", true, longestAcceptableAppID, "/path", ""},
		{longestAcceptableAppID + "v/path", false, "", "/path", ""},
		{"myapp/myfn/with/long/path", true, "myapp", "/myfn/with/long/path", ""},
		{"myapp/myfn /spaces", false, "", "", ""},
	}

	for _, c := range cases {
		parsed, err := ParseFunctionID(c.fnID)
		if err != nil && c.works {
			t.Errorf("Expecting %s to parse, but does not", c.fnID)
			continue
		}
		if err == nil && !c.works {
			t.Errorf("Expecting %s not to parse, but did", c.fnID)
			continue
		}
		if !c.works {
			continue
		}

		require.NotNil(t, parsed)
		assert.Equal(t, c.appID, parsed.AppID)
		assert.Equal(t, c.path, parsed.Path)
		assert.Equal(t, c.query, parsed.Query)

	}
}
