package server

import (
	"github.com/fnproject/flow/model"
	"regexp"
)

var graphIDRegex = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

var stageIDRegex = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

func validFunctionID(functionID string, allowRelative bool) bool {
	fn, err := model.ParseFunctionID(functionID)

	if err != nil {
		return false
	}
	if fn.IsRelative() && !allowRelative {
		return false
	}
	return true
}

func validGraphID(graphID string) bool {
	return graphIDRegex.MatchString(graphID)
}

func validStageID(stageID string) bool {
	return stageIDRegex.MatchString(stageID)
}
