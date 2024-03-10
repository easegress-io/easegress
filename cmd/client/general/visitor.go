/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package general provides the general utilities for the client.
package general

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// YAMLVisitor walk through multiple YAML documents
type YAMLVisitor interface {
	Visit(func(yamlDoc []byte) error) error
	Close()
}

type yamlVisitor struct {
	reader io.Reader
}

// Visit implements YAMLVisitor
func (v *yamlVisitor) Visit(fn func(yamlDoc []byte) error) error {
	r := yaml.NewYAMLReader(bufio.NewReader(v.reader))

	for {
		data, err := r.Read()
		if len(data) == 0 {
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			continue
		}
		if err = fn(data); err != nil {
			return err
		}
	}
}

// Close closes the yamlVisitor
func (v *yamlVisitor) Close() {
	if closer, ok := v.reader.(io.Closer); ok {
		closer.Close()
	}
}

// Spec is the spec of a resource
type Spec struct {
	Kind string
	Name string
	doc  string
}

func (s *Spec) Doc() string {
	return s.doc
}

// SpecVisitor walk through multiple specs
type SpecVisitor interface {
	Visit(func(*Spec) error) error
	Close()
}

type specVisitor struct {
	v YAMLVisitor
}

// Visit implements SpecVisitor
func (v *specVisitor) Visit(fn func(*Spec) error) error {
	var specs []Spec

	err := v.v.Visit(func(yamlDoc []byte) error {
		s := Spec{}
		doc := string(yamlDoc)

		err := yaml.Unmarshal(yamlDoc, &s)
		if err != nil {
			return fmt.Errorf("error parsing %s: %v", doc, err)
		}

		if s.Kind == "" {
			return fmt.Errorf("kind is empty: %s", doc)
		}

		s.doc = doc
		specs = append(specs, s)
		return nil
	})

	if err != nil {
		ExitWithError(err)
	}

	for _, s := range specs {
		fn(&s)
	}

	return nil
}

// Close closes the specVisitor
func (v *specVisitor) Close() {
	v.v.Close()
}

// BuildYAMLVisitor builds a YAMLVisitor
func BuildYAMLVisitor(yamlFile string, cmd *cobra.Command) YAMLVisitor {
	var r io.ReadCloser
	if yamlFile == "-" {
		r = io.NopCloser(os.Stdin)
	} else if f, err := os.Open(yamlFile); err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	} else {
		r = f
	}
	return &yamlVisitor{reader: r}
}

// BuildSpecVisitor builds a SpecVisitor
func BuildSpecVisitor(yamlFile string, cmd *cobra.Command) SpecVisitor {
	v := BuildYAMLVisitor(yamlFile, cmd)
	return &specVisitor{v: v}
}

func GetSpecFromYaml(yamlStr string) (*Spec, error) {
	s := Spec{}
	err := yaml.Unmarshal([]byte(yamlStr), &s)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %v", yamlStr, err)
	}
	s.doc = yamlStr
	return &s, nil
}

// CompareYamlNameKind compares the name and kind of two YAML strings
func CompareYamlNameKind(oldYaml, newYaml string) (*Spec, *Spec, error) {
	s1, err := GetSpecFromYaml(oldYaml)
	if err != nil {
		return nil, nil, err
	}

	s2, err := GetSpecFromYaml(newYaml)
	if err != nil {
		return nil, nil, err
	}

	if s1.Kind != s2.Kind {
		return nil, nil, fmt.Errorf("kind is not equal: %s, %s", s1.Kind, s2.Kind)
	}
	if s1.Name != s2.Name {
		return nil, nil, fmt.Errorf("name is not equal: %s, %s", s1.Name, s2.Name)
	}
	return s1, s2, nil
}
