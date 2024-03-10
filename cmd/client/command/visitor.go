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

// Package command provides the commands.
package command

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/megaease/easegress/v2/cmd/client/general"
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

type spec struct {
	Kind string
	Name string
	doc  string
}

// SpecVisitor walk through multiple specs
type SpecVisitor interface {
	Visit(func(*spec) error) error
	Close()
}

type specVisitor struct {
	v YAMLVisitor
}

// Visit implements SpecVisitor
func (v *specVisitor) Visit(fn func(*spec) error) error {
	var specs []spec

	err := v.v.Visit(func(yamlDoc []byte) error {
		s := spec{}
		doc := string(yamlDoc)

		err := yaml.Unmarshal(yamlDoc, &s)
		if err != nil {
			return fmt.Errorf("error parsing %s: %v", doc, err)
		}

		if s.Name == "" {
			return fmt.Errorf("name is empty: %s", doc)
		}

		if s.Kind == "" {
			return fmt.Errorf("kind is empty: %s", doc)
		}

		s.doc = doc
		specs = append(specs, s)
		return nil
	})

	if err != nil {
		general.ExitWithError(err)
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

func buildYAMLVisitor(yamlFile string, cmd *cobra.Command) YAMLVisitor {
	var r io.ReadCloser
	if yamlFile == "" {
		r = io.NopCloser(os.Stdin)
	} else if f, err := os.Open(yamlFile); err != nil {
		general.ExitWithErrorf("%s failed: %v", cmd.Short, err)
	} else {
		r = f
	}
	return &yamlVisitor{reader: r}
}

func buildSpecVisitor(yamlFile string, cmd *cobra.Command) SpecVisitor {
	v := buildYAMLVisitor(yamlFile, cmd)
	return &specVisitor{v: v}
}
