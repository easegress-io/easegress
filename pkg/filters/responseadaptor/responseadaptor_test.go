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

package responseadaptor

import (
	"io"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestResponseAdaptor(t *testing.T) {
	yamlSpec := `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "X-Mock": "mockedHeaderValue" 
body: "copyright"
`
	ra := doTest(t, yamlSpec, nil)

	yamlSpec = `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "X-Mock": "mockedHeaderValue"
body: "copyright"
`
	doTest(t, yamlSpec, ra)
}

func doTest(t *testing.T, yamlSpec string, prev *ResponseAdaptor) *ResponseAdaptor {
	assert := assert.New(t)
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	ra := kind.CreateInstance(spec)
	if prev == nil {
		ra.Init()
	} else {
		ra.Inherit(prev)
	}

	ctx := context.New(nil)
	resp, err := httpprot.NewResponse(nil)
	assert.Nil(err)
	ctx.SetResponse("resp", resp)
	ctx.UseResponse("resp")

	resp.Std().Header.Add("X-Del", "deleted")

	ra.Handle(ctx)
	assert.Equal("mockedHeaderValue", resp.Std().Header.Get("X-Mock"))
	assert.Equal("", resp.Std().Header.Get("X-Del"))

	body, err := io.ReadAll(resp.GetPayload())
	assert.Nil(err)
	assert.Equal("copyright", string(body))

	ra.Status()
	return ra.(*ResponseAdaptor)
}
