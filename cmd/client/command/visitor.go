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

// VisitorFunc executes visition logic
type VisitorFunc func(*spec)

// Visitor walk through the document via VisitorFunc
type Visitor interface {
	Visit(VisitorFunc)
}

type spec struct {
	Kind string
	Name string
	doc  string
}

// StreamVisitor is the struct of Visitor Pattern
type StreamVisitor struct {
	io.Reader
}

// NewStreamVisitor returns a streamVisitor.
func NewStreamVisitor(src string) *StreamVisitor {
	return &StreamVisitor{
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

// Visit implements Visitor over a stream.
func (v *StreamVisitor) Visit(fn VisitorFunc) {
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
		s.doc = d.doc
		//TODO can validate spec's Kind here
		validSpecs = append(validSpecs, s)
	}
	for _, s := range validSpecs {
		fn(&s)
	}
}
