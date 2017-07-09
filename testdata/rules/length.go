package main

import (
	es "github.com/patrobinson/go-fish/testdata/eventStructs"
)

type lengthRule string

func (r lengthRule) Process(thing interface{}) bool {
	foo, ok := thing.(es.ExampleType)
	if ok && len(foo.Str) == 1 {
		return true
	}
	return false
}

func (r lengthRule) String() string { return string(r) }

var Rule lengthRule
