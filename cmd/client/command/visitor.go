/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package command

import (
	"bufio"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// SpecVisitorFunc executes visition logic
type SpecVisitorFunc func(*spec)

// SpecVisitor walk through the document via SpecVisitorFunc
type SpecVisitor interface {
	Visit(SpecVisitorFunc)
}

type spec struct {
	Kind string
	Name string
	doc  string
}

type specVisitor struct {
	io.Reader
}

// NewSpecVisitor returns a spec visitor.
func NewSpecVisitor(src string) SpecVisitor {
	return &specVisitor{
		Reader: strings.NewReader(src),
	}
}

type yamlDecoder struct {
	reader *yaml.YAMLReader
	doc    string
}

func newYAMLDecoder(r io.Reader) *yamlDecoder {
	return &yamlDecoder{
		reader: yaml.NewYAMLReader(bufio.NewReader(r)),
	}
}

// Decode reads a YAML document into bytes and tries to yaml.Unmarshal it.
func (d *yamlDecoder) Decode(into interface{}) error {
	bytes, err := d.reader.Read()
	if err != nil && err != io.EOF {
		return err
	}
	d.doc = string(bytes)
	if len(bytes) != 0 {
		err = yaml.Unmarshal(bytes, into)
	}
	return err
}

// Visit implements SpecVisitor
func (v *specVisitor) Visit(fn SpecVisitorFunc) {
	d := newYAMLDecoder(v.Reader)
	var validSpecs []spec
	for {
		var s spec
		if err := d.Decode(&s); err != nil {
			if err == io.EOF {
				break
			} else {
				ExitWithErrorf("error parsing %s: %v", d.doc, err)
			}
		}
		if s.Name == "" {
			ExitWithErrorf("name is empty: %s", d.doc)
		}
		if s.Kind == "" {
			ExitWithErrorf("kind is empty: %s", d.doc)
		}
		s.doc = d.doc
		//TODO can validate spec's Kind here
		validSpecs = append(validSpecs, s)
	}
	for _, s := range validSpecs {
		fn(&s)
	}
}
