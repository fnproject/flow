package server

import "regexp"

var fnIdRegex = regexp.MustCompile("^[a-zA-Z0-9_\\-]{1,255}/[a-zA-Z0-9_/\\-]{0,255}$")

var graphIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")

var stageIdRegx = regexp.MustCompile("^[a-zA-Z0-9\\-_]{1,255}$")


func validFunctionId(functionId string) bool{
	return fnIdRegex.MatchString(functionId)
}


func validGraphId(graphIdId string) bool{
	return graphIdRegx.MatchString(graphIdId)
}


func validStageId(stageId string) bool{
	return stageIdRegx.MatchString(stageId)
}