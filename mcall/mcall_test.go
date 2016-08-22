package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"fmt"
)

func TestMap(t *testing.T) {
	var args = Args{"i": "ls -al", "loglevel": "DEBUG"}
	rslt := mainExec(args)
	if assert.NotNil(t, rslt) {
		fmt.Printf("result %v", rslt)
		//	assert.Equal(t, result, len(rslt), "at least test should pass")
	}
}
