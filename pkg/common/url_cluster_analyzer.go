package common

import (
	"strings"
	"sync"
)

const (
	maxValues = 20
	maxLayers = 256
)

type URLClusterAnalyzer struct {
	slots []*field `yaml:"slots"`
	mutex *sync.Mutex
}

type field struct {
	constant        string   `yaml:"constant""`
	subFields       []*field `yaml:"subFields"`
	variableField   *field   `yaml:"variableField"`
	isVariableField bool     `yaml:"isVariableField"`
	pattern         string   `yaml:"pattern"`
}

func newField(name string) *field {
	f := &field{
		constant:        name,
		subFields:       make([]*field, 0),
		variableField:   nil,
		isVariableField: false,
		pattern:         "",
	}

	return f
}

func NewUrlClusterAnalyzer() *URLClusterAnalyzer {
	u := &URLClusterAnalyzer{
		mutex: &sync.Mutex{},
		slots: make([]*field, maxLayers),
	}

	for i := 0; i < maxLayers; i++ {
		u.slots[i] = newField("root")
	}
	return u
}

// Extract the pattern of a Restful url path.
//  A field of the path occurs more than 20 distinct values will be consider as a variables.
//  e.g input: /com/megaease/users/123/friends/456
//  output: /com/megaease/users/*/friends/*
func (u *URLClusterAnalyzer) GetPattern(urlPath string) string {
	if urlPath == "" {
		return ""
	}

	var values []string
	if urlPath[0] == '/' {
		values = strings.Split(urlPath[1:], "/")
	} else {
		values = strings.Split(urlPath, "/")
	}

	pos := len(values)
	if len(values) >= maxLayers {
		pos = maxLayers - 1

	}
	currField := u.slots[pos]
	currPattern := currField.pattern

	u.mutex.Lock()
	defer u.mutex.Unlock()

LOOP:
	for i := 0; i < len(values); i++ {

		// a known variable field
		if currField.isVariableField {
			currPattern = currField.pattern
			currField = currField.variableField
			continue
		}

		// a known constant field
		for _, v := range currField.subFields {
			if v.constant == values[i] {
				currPattern = v.pattern
				currField = v
				continue LOOP
			}
		}

		newF := newField(values[i])
		currField.subFields = append(currField.subFields, newF)

		// find out a new variable field
		if len(currField.subFields) > maxValues {
			currField.isVariableField = true
			currField.variableField = newField("*")
			currField.pattern = currPattern + "/*"

			currPattern = currField.pattern

			currField = currField.variableField
			continue
		}

		// find out a new constant field
		newF.pattern = currPattern + "/" + values[i]
		currPattern = newF.pattern
		currField = newF
	}

	return currPattern
}
