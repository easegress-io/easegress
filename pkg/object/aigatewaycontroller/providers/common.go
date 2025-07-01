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

package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/httphelper"
)

var ErrMetricInternalServer = errors.New("internal_server_err")
var ErrMetricUnmarshalResponse = errors.New("unmarshal_response_err")
var ErrMetricUnmarshalRequest = errors.New("unmarshal_request_err")

type (
	// RequestMapper is a function that maps user OpenAI request to a provider request.
	RequestMapper func(pc *ProviderContext) (path string, newBody []byte, err error)

	ProviderContext struct {
		Ctx      *context.Context
		Provider *ProviderSpec

		Req       *httpprot.Request
		ReqBody   []byte
		ReqInfo   *protocol.GeneralRequest
		OpenAIReq map[string]any

		Resp     *httpprot.Response
		RespType ResponseType

		callBacks []func(pc *ProviderContext, fc *FinishContext)
	}

	FinishContext struct {
		Resp     *http.Response
		RespBody []byte
		Duration int64
		Error    error
	}
)

func NewProviderContext(ctx *context.Context, provider *ProviderSpec, req *httpprot.Request, resp *httpprot.Response) (*ProviderContext, error) {
	// request body
	body, err := io.ReadAll(req.GetPayload())
	if err != nil {
		return nil, err
	}

	// parse OpenAI request
	openAIReq := make(map[string]interface{})
	err = json.Unmarshal(body, &openAIReq)
	if err != nil {
		return nil, err
	}

	// parse model and stream
	model := openAIReq["model"].(string)
	streamStr := openAIReq["stream"]
	stream := false
	if streamStr != nil {
		stream, _ = strconv.ParseBool(streamStr.(string))
	}

	path := req.URL().Path
	respType := ResponseType("")
	if strings.HasSuffix(path, string(ResponseTypeChatCompletions)) {
		respType = ResponseTypeChatCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeCompletions)) {
		respType = ResponseTypeCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeEmbeddings)) {
		respType = ResponseTypeEmbeddings
	} else if strings.HasSuffix(path, string(ResponseTypeModels)) {
		respType = ResponseTypeModels
	} else if strings.HasSuffix(path, string(ResponseTypeImageGenerations)) {
		respType = ResponseTypeImageGenerations
	} else {
		return nil, fmt.Errorf("unsupported request path: %s", path)
	}

	c := &ProviderContext{
		Ctx:       ctx,
		Provider:  provider,
		Req:       req,
		ReqBody:   body,
		OpenAIReq: openAIReq,
		ReqInfo: &protocol.GeneralRequest{
			Model:  model,
			Stream: stream,
		},
		Resp:     resp,
		RespType: respType,
	}
	return c, nil
}

func (pc *ProviderContext) AddCallBack(fn func(*ProviderContext, *FinishContext)) {
	pc.callBacks = append(pc.callBacks, fn)
}

func (pc *ProviderContext) Finish(fc *FinishContext) {
	for _, fn := range pc.callBacks {
		fn(pc, fc)
	}
}

func prepareRequest(pc *ProviderContext, mapper RequestMapper) (request *http.Request, err error) {
	u, err := url.Parse(pc.Provider.BaseURL)
	if err != nil {
		return nil, err
	}

	path, newBody, err := mapper(pc)
	if err != nil {
		return nil, err
	}

	u.Path = path
	if pc.Provider.APIVersion != "" {
		query := u.Query()
		query.Set("api-version", pc.Provider.APIVersion)
		u.RawQuery = query.Encode()
	}
	u.RawQuery = pc.Req.URL().RawQuery
	req, err := http.NewRequestWithContext(pc.Req.Context(), pc.Req.Method(), u.String(), bytes.NewReader(newBody))
	if err != nil {
		return nil, err
	}

	headers := pc.Req.HTTPHeader()
	httphelper.RemoveHopByHopHeaders(headers)
	maps.Copy(req.Header, headers)

	if pc.Provider.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pc.Provider.APIKey))
	}
	for k, v := range pc.Provider.Headers {
		req.Header.Set(k, v)
	}
	return req, err
}

func setEgErrResponse(resp *httpprot.Response, code int, err error) {
	resp.SetStatusCode(code)
	errMsg := protocol.NewError(code, err.Error())
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
}

func setEgResponseFromResponse(egResp *httpprot.Response, resp *http.Response, body []byte) {
	egResp.SetStatusCode(resp.StatusCode)
	header := resp.Header
	httphelper.RemoveHopByHopHeaders(header)
	maps.Copy(egResp.HTTPHeader(), header)
	egResp.SetPayload(body)
}
