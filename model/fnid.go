package model

import (
	"fmt"
	"regexp"
)

type FunctionID struct {
	AppId string
	Path  string
	// Query including ? if present
	Query string
}

func (fid *FunctionID) IsRelative() bool {
	return fid.AppId == "."
}

func (fid *FunctionID) String() string {
	return fid.AppId + fid.Path + fid.Query
}

var fnIdRegex = regexp.MustCompile("^(\\.|[a-zA-Z0-9_\\-]{1,255})(/[a-zA-Z0-9_/\\-]{0,255})(\\?.*)?$")

func ParseFunctionId(fnId string) (*FunctionID, error) {

	res := fnIdRegex.FindStringSubmatch(fnId)

	if res == nil {
		return nil, fmt.Errorf("invalid function ID")
	}

	return &FunctionID{
		AppId: res[1],
		Path:  res[2],
		Query: res[3],
	}, nil
}
