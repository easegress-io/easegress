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

package httpbuilder

import (
	"bytes"
	"text/template"
	"text/template/parse"
)

type builder struct {
	template *template.Template
	value    string
}

func newBuilder(text string) *builder {
	temp := template.Must(template.New(text).Parse(text))
	for _, node := range temp.Root.Nodes {
		if node.Type() == parse.NodeAction {
			return &builder{temp, ""}
		}
	}
	return &builder{nil, text}
}

func (b *builder) build(data interface{}) ([]byte, error) {
	if b.template == nil {
		return []byte(b.value), nil
	}

	var result bytes.Buffer
	err := b.template.Execute(&result, data)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func (b *builder) buildString(data interface{}) (string, error) {
	if b.template == nil {
		return b.value, nil
	}

	var result bytes.Buffer
	err := b.template.Execute(&result, data)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}
