package server

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestGraphIdValidation(t *testing.T) {

	cases := []struct {
		g string
		v bool
	}{
		{"", false},
		{" ", false},
		{" s", false},
		{"s ", false},
		{"&&&", false},
		{"graphId", true},
		{"87786152-8821-11e7-acc3-b7dd555ee2ee", true}}

	for _, c := range cases {
		assert.Equal(t, c.v, validGraphId(c.g), "Case %s", c.g)
	}
}



func TestStageIdValidation(t *testing.T) {

	cases := []struct {
		g string
		v bool
	}{
		{"", false},
		{" ", false},
		{" s", false},
		{"s ", false},
		{"&&&", false},
		{"graphId", true},
		{"87786152-8821-11e7-acc3-b7dd555ee2ee", true}}

	for _, c := range cases {
		assert.Equal(t, c.v, validGraphId(c.g), "Case %s", c.g)
	}
}

func TestFunctionIdValidation(t *testing.T) {

	cases := []struct {
		g string
		v bool
	}{
		{"", false},
		{" ", false},
		{" s", false},
		{"s ", false},
		{"&&&", false},
		{"app/", true},
		{"Myapp/", true},
		{"myapp/myfn", true},
		{"myapp/myfn/with/long/path", true},
		{"myapp/myfn /spaces", false},
	}

	for _, c := range cases {
		assert.Equal(t, c.v, validFunctionId(c.g), "Case %s", c.g)
	}
}