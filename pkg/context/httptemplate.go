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

package context

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/texttemplate"
)

const (
	filterReqPath       = "filter.%s.req.path"
	filterReqMethod     = "filter.%s.req.method"
	filterReqBody       = "filter.%s.req.body"
	filterReqScheme     = "filter.%s.req.scheme"
	filterReqProto      = "filter.%s.req.proto"
	filterReqhost       = "filter.%s.req.host"
	filterReqheader     = "filter.%s.req.header.%s"
	filterRspStatusCode = "filter.%s.rsp.statuscode"
	filterRspBody       = "filter.%s.rsp.body"

	defaultMaxBodySize = 10240
	defaultTagNum      = 4

	filterNameTagIndex   = 1
	filterReqRspTagIndex = 2
	filterValueTagIndex  = 3
)

type (
	// HTTPTemplate is the wrapper of template engine for Easegress
	HTTPTemplate struct {
		Engine        texttemplate.TemplateEngine
		metaTemplates []string

		// store the filter name and its
		// calling function lists
		filterExecFuncs map[string]filterDictFuncs

		// the dependency order array of filters
		filtersOrder []string
	}

	setDictFunc func(*HTTPTemplate, string, HTTPContext) error

	filterDictFuncs struct {
		reqFuncs []setDictFunc
		rspFuncs []setDictFunc
	}

	// FilterBuff stores filter's name and its YAML bytes
	FilterBuff struct {
		Name string
		Buff []byte
	}
)

var (
	metaTemplates = []string{
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.path",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	}

	tagsFuncMap = map[string]setDictFunc{
		"req.method":     saveReqMethod,
		"req.body":       saveReqBody,
		"req.path":       saveReqPath,
		"req.scheme":     saveReqScheme,
		"req.proto":      saveReqProto,
		"req.host":       saveReqHost,
		"req.header":     saveReqHeader,
		"rsp.statuscode": saveRspStatuscode,
		"rsp.body":       saveRspBody,
	}
)

// NewHTTPTemplate returns a default HTTPTemplate
func NewHTTPTemplate(filterBuffs []FilterBuff) (*HTTPTemplate, error) {
	engine, err := texttemplate.NewDefault(metaTemplates)
	if err != nil {
		logger.Errorf("init http template fail [%v]", err)
		return nil, err
	}

	e := HTTPTemplate{
		Engine:          engine,
		metaTemplates:   metaTemplates,
		filterExecFuncs: map[string]filterDictFuncs{},
	}

	filterFuncTags := map[string][]string{}
	// validates the filter's YAML spec for dependency checking
	// and template format,e.g., if filter1 has a template said '[[filter.filter2.rsp.data]],
	// but it appears before filter2, then it's an invalidated dependency cause we can't get
	// the rsp form filter2 in the execution period of filter1. At last it will build up
	// executing function arrays for every filter.
	for _, filterBuff := range filterBuffs {
		e.filtersOrder = append(e.filtersOrder, filterBuff.Name)
		templatesMap := e.Engine.ExtractRawTemplateRuleMap(string(filterBuff.Buff))
		if len(templatesMap) == 0 {
			continue
		}
		dependFilters := []string{}
		for template, renderMeta := range templatesMap {
			// no matched and rendered meta template
			if len(renderMeta) == 0 {
				err = fmt.Errorf("filter %s template [[%s]] check failed, unregonized", filterBuff.Name, template)
				break
			}

			tags := strings.Split(renderMeta, texttemplate.DefaultSeparator)
			if len(tags) < defaultTagNum {
				err = fmt.Errorf("filter %s template [[%s]] check failed,its render metaTemplate [[%s]] is invalid",
					filterBuff.Name, template, renderMeta)
				break
			} else {
				dependFilterName := tags[filterNameTagIndex]
				dependFilters = append(dependFilters, dependFilterName)
				funcTag := tags[filterReqRspTagIndex] + texttemplate.DefaultSeparator +
					tags[filterValueTagIndex]

				funcTags := filterFuncTags[dependFilterName]
				found := false
				for _, v := range funcTags {
					if funcTag == v {
						found = true
						break
					}
				}
				if !found {
					filterFuncTags[dependFilterName] = append(funcTags, funcTag)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		// get its all rely filters and make sure these targets have already show
		// up, and couldn't rely on itself.
		if err = e.validateFilterDependency(filterBuff.Name, dependFilters); err != nil {
			return nil, err
		}
	}

	e.storeFilterExecFuncs(filterFuncTags)

	return &e, nil
}

// NewHTTPTemplateDummy return a empty implement version of HTTP template
func NewHTTPTemplateDummy() *HTTPTemplate {
	return &HTTPTemplate{
		Engine:          texttemplate.NewDummyTemplate(),
		filterExecFuncs: map[string]filterDictFuncs{},
	}
}

func readBody(body io.Reader, maxBodySize int64) (*bytes.Buffer, error) {
	buff := bytes.NewBuffer(nil)
	if body == nil {
		return buff, nil
	}
	written, err := io.CopyN(buff, body, defaultMaxBodySize+1)

	if err != nil && err != io.EOF {
		err = fmt.Errorf("read body failed: %v", err)
		return nil, err
	}

	if written > defaultMaxBodySize {
		err = fmt.Errorf("body exceed %dB", defaultMaxBodySize)
		return nil, err
	}

	return buff, nil
}

// SaveRequest transforms HTTPRequest related fields into template engine's dictionary
func (e *HTTPTemplate) SaveRequest(filterName string, ctx HTTPContext) error {
	var (
		execFuncs filterDictFuncs
		ok        bool
	)
	if execFuncs, ok = e.filterExecFuncs[filterName]; !ok {
		return nil
	}

	if len(execFuncs.reqFuncs) == 0 {
		return nil
	}

	for _, f := range execFuncs.reqFuncs {
		if err := f(e, filterName, ctx); err != nil {
			logger.Errorf("filter %s execute HTTP template req func %v failed err %v",
				filterName, f, err)
			return err
		}
	}

	return nil
}

// SaveResponse transforms HTTPResponse related fields into template engine's dictionary
func (e *HTTPTemplate) SaveResponse(filterName string, ctx HTTPContext) error {
	var (
		execFuncs filterDictFuncs
		ok        bool
	)
	if execFuncs, ok = e.filterExecFuncs[filterName]; !ok {
		return nil
	}

	if len(execFuncs.rspFuncs) == 0 {
		return nil
	}

	for _, f := range execFuncs.rspFuncs {
		if err := f(e, filterName, ctx); err != nil {
			logger.Errorf("filter %s execute HTTP template rsp func %v failed err %v",
				filterName, f, err)
			return err
		}
	}

	return nil
}

// Render using engine to render template
func (e *HTTPTemplate) Render(input string) (string, error) {
	return e.Engine.Render(input)
}

func (e *HTTPTemplate) validateFilterDependency(filterName string, dependFilters []string) error {
	var err error
	for _, name := range dependFilters {
		if filterName == name {
			err = fmt.Errorf("filter %s template check failed, rely on itself", filterName)
			break
		}
		found := false
		for _, existFilter := range e.filtersOrder {
			if existFilter == name {
				found = true
				break
			}
		}

		if !found {
			err = fmt.Errorf("filter %s template check failed, rely on a havn't executed filter %s ",
				filterName, name)
			break
		}
	}
	return err
}

func (e *HTTPTemplate) storeFilterExecFuncs(filterFuncTags map[string][]string) {
	for filterName, funcTags := range filterFuncTags {
		var (
			execFuncs filterDictFuncs
			ok        bool
		)
		if execFuncs, ok = e.filterExecFuncs[filterName]; !ok {
			e.filterExecFuncs[filterName] = filterDictFuncs{}
		}

		for _, funcTag := range funcTags {
			if f, ok := tagsFuncMap[funcTag]; ok {
				if strings.Contains(funcTag, "req") {
					execFuncs.reqFuncs = append(execFuncs.reqFuncs, f)
				} else {
					execFuncs.rspFuncs = append(execFuncs.rspFuncs, f)
				}
			}
		}
		e.filterExecFuncs[filterName] = execFuncs
	}
}

func saveRspStatuscode(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterRspStatusCode, filterName), strconv.Itoa(ctx.Response().StatusCode()))
}

func saveReqHost(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterReqhost, filterName), ctx.Request().Host())
}

func saveReqPath(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterReqPath, filterName), ctx.Request().Path())
}

func saveReqProto(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterReqProto, filterName), ctx.Request().Proto())
}

func saveReqScheme(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterReqScheme, filterName), ctx.Request().Scheme())
}

func saveReqMethod(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(filterReqMethod, filterName), ctx.Request().Method())
}

func saveReqBody(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Request().Body(), defaultMaxBodySize)
	if err != nil {
		logger.Errorf("httptemplate save HTTP request  body failed err %v", err)
		return err
	}
	e.Engine.SetDict(fmt.Sprintf(filterReqBody, filterName), string(bodyBuff.Bytes()))
	// Reset back into Request's Body
	ctx.Request().SetBody(bodyBuff)

	return nil
}

func saveReqHeader(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	// Set the HTTP request Header
	for k, v := range ctx.Request().Std().Header {
		if len(v) > 0 {
			if len(v) == 1 {
				e.Engine.SetDict(fmt.Sprintf(filterReqheader, filterName, k), v[0])
			} else {
				// one header field with multiple values, join them with "," according to
				// https://stackoverflow.com/q/3096888/1705845
				e.Engine.SetDict(fmt.Sprintf(filterReqheader, filterName, k), strings.Join(v, ","))
			}
		}
	}
	return nil
}

func saveRspBody(e *HTTPTemplate, filterName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Response().Body(), defaultMaxBodySize)
	if err != nil {
		logger.Errorf("httptemplate save HTTP response body failed err %v", err)
		return err
	}
	e.Engine.SetDict(fmt.Sprintf(filterRspBody, filterName), string(bodyBuff.Bytes()))
	ctx.Response().SetBody(bodyBuff)
	return nil
}
