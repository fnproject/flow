package server

import (
	"regexp"
	"github.com/fnproject/completer/model"
)

var graphIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

var stageIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")


func validFunctionId(functionId string,allowRelative bool) bool{
	fn,err := model.ParseFunctionId(functionId)

	if err !=nil {
		return false
	}
	if fn.IsRelative() && !allowRelative {
		return false
	}
	return true
}


func validGraphId(graphIdId string) bool{
	return graphIdRegx.MatchString(graphIdId)
}


func validStageId(stageId string) bool{
	return stageIdRegx.MatchString(stageId)
}