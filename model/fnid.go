package model

import (
	"fmt"
	"regexp"
)

// FunctionID An holder for a function ID (by parts)
// can be parsed and stringified
type FunctionID struct {
	AppID string
	Path  string
	// Query including ? if present
	Query string
}

// IsRelative : is this function ID relative  or absolute
func (fid *FunctionID) IsRelative() bool {
	return fid.AppID == "."
}

func (fid *FunctionID) String() string {
	return fid.AppID + fid.Path + fid.Query
}

var fnIDRegex = regexp.MustCompile("^(\\.|[a-zA-Z0-9_\\-]{1,255})(/[a-zA-Z0-9_/\\-]{0,255})(\\?.*)?$")

// ParseFunctionID extracts a function ID from a string
func ParseFunctionID(fnID string) (*FunctionID, error) {

	res := fnIDRegex.FindStringSubmatch(fnID)

	if res == nil {
		return nil, fmt.Errorf("invalid function ID")
	}

	return &FunctionID{
		AppID: res[1],
		Path:  res[2],
		Query: res[3],
	}, nil
}
