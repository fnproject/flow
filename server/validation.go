package server

import (
	"github.com/fnproject/flow/model"
	"regexp"
)

var graphIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

var stageIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

func validFunctionId(functionId string, allowRelative bool) bool {
	fn, err := model.ParseFunctionId(functionId)

	if err != nil {
		return false
	}
	if fn.IsRelative() && !allowRelative {
		return false
	}
	return true
}

func validGraphId(graphId string) bool {
	return graphIdRegx.MatchString(graphId)
}

func validStageId(stageId string) bool {
	return stageIdRegx.MatchString(stageId)
}
