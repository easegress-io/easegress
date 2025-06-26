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
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/httphelper"
)

var ErrInternalServer = errors.New("internal_server_err")
var ErrUnmarshalResponse = errors.New("unmarshal_response_err")
var ErrUnmarshalRequest = errors.New("unmarshal_request_err")

type (
	ProviderContext struct {
		Ctx      *context.Context
		Provider *ProviderSpec
		Req      *httpprot.Request
		Resp     *httpprot.Response
	}

	// RequestMapper is a function that maps user OpenAI request to a provider request.
	RequestMapper func(req *httpprot.Request, body []byte) (path string, config *protocol.GeneralRequest, newBody []byte, err error)
)

func prepareRequest(pc *ProviderContext, mapper RequestMapper) (request *http.Request, config *protocol.GeneralRequest, err error) {
	u, err := url.Parse(pc.Provider.BaseURL)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(pc.Req.GetPayload())
	if err != nil {
		return nil, nil, err
	}

	path, stream, newBody, err := mapper(pc.Req, body)
	if err != nil {
		return nil, nil, err
	}

	u.Path = path
	u.RawQuery = pc.Req.URL().RawQuery
	req, err := http.NewRequestWithContext(pc.Req.Context(), pc.Req.Method(), u.String(), bytes.NewReader(newBody))
	if err != nil {
		return nil, nil, err
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
	return req, stream, err
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
