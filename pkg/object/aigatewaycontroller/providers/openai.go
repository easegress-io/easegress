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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

type (
	OpenAIProvider struct {
		providerSpec *ProviderSpec
	}
)

func (p *OpenAIProvider) Type() string {
	return "openai"
}

func (p *OpenAIProvider) Handle(ctx *context.Context, req *httpprot.Request, resp *httpprot.Response) string {
	providerContext := &ProviderContext{
		Ctx:      ctx,
		Req:      req,
		Resp:     resp,
		Provider: p.providerSpec,
	}
	request, isStream, err := prepareRequest(providerContext, openaiRequestMapper)
	if err != nil {
		setErrResponse(resp, http.StatusInternalServerError, err)
		return ResultInternalError
	}
	return openaiProxyRequest(providerContext, request, isStream)
}

func openaiProxyRequest(pc *ProviderContext, req *http.Request, isStream bool) string {
	// TODO
	return ""
}

// openaiRequestMapper is a RequestMapper to map user request to OpenAI provider request.
func openaiRequestMapper(req *httpprot.Request, body []byte) (string, bool, []byte, error) {
	// remove the routing prefix from the request path
	path := req.URL().Path
	if strings.HasSuffix(path, "/v1/chat/completions") {
		path = "/v1/chat/completions"
	} else if strings.HasSuffix(path, "/v1/completions") {
		path = "/v1/completions"
	} else {
		return "", false, nil, fmt.Errorf("unsupported OpenAI request path: %s", path)
	}

	openaiReq := &protocol.GeneralRequest{}
	if err := json.Unmarshal(body, openaiReq); err != nil {
		return "", false, nil, fmt.Errorf("failed to unmarshal OpenAI request: %w", err)
	}
	return path, openaiReq.Stream, body, nil
}
