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
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/httphelper"
	"github.com/megaease/easegress/v2/pkg/util/readers"
)

// BaseProvider is a struct that contains the common fields for all AI gateway providers.
// Almost all providers compatible with OpenAI API, so we abstract the common logic.
type BaseProvider struct {
	providerSpec *ProviderSpec
}

var _ Provider = (*BaseProvider)(nil)

func (bp *BaseProvider) Type() string {
	return bp.providerSpec.ProviderType
}

func (bp *BaseProvider) Handle(ctx *context.Context, req *httpprot.Request, resp *httpprot.Response, updateMetricFn func(*metricshub.Metric)) string {
	providerContext := &ProviderContext{
		Ctx:      ctx,
		Req:      req,
		Resp:     resp,
		Provider: bp.providerSpec,
	}
	request, openaiReq, err := prepareRequest(providerContext, bp.RequestMapper)
	if err != nil {
		setEgErrResponse(resp, http.StatusInternalServerError, err)
		return ResultInternalError
	}
	return bp.ProxyRequest(providerContext, openaiReq, request, updateMetricFn)
}

func (bp *BaseProvider) RequestMapper(req *httpprot.Request, body []byte) (string, *protocol.GeneralRequest, []byte, error) {
	// remove the routing prefix from the request path
	path := req.URL().Path
	if strings.HasSuffix(path, "/v1/chat/completions") {
		path = "/v1/chat/completions"
	} else if strings.HasSuffix(path, "/v1/completions") {
		path = "/v1/completions"
	} else {
		return "", nil, nil, fmt.Errorf("unsupported %s request path: %s", bp.Type(), path)
	}

	openaiReq := &protocol.GeneralRequest{}
	if err := json.Unmarshal(body, openaiReq); err != nil {
		return "", nil, nil, fmt.Errorf("failed to unmarshal %s request: %w", bp.Type(), err)
	}
	return path, openaiReq, body, nil
}

func (bp *BaseProvider) ProxyRequest(pc *ProviderContext, openaiReq *protocol.GeneralRequest, req *http.Request, updateMetricFn func(*metricshub.Metric)) string {
	// TODO add metrics
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setEgErrResponse(pc.Resp, http.StatusInternalServerError, err)
		return ResultInternalError
	}
	// cant use defer to close body here.
	// for stream data, we need to return this function and use a goroutine to send data and close body after than.
	duration := time.Since(start).Milliseconds()

	// copy response data to buf
	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)
	body := readers.NewCallbackReader(tee)
	body.OnClose(func() {
		resp.Body.Close()
	})
	body.OnClose(func() {
		metric := metricshub.Metric{
			Success:      resp.StatusCode == http.StatusOK,
			Duration:     duration,
			Provider:     pc.Provider.Name,
			ProviderType: pc.Provider.ProviderType,
			Model:        openaiReq.Model,
			BaseURL:      pc.Provider.BaseURL,
		}
		inputToken, outputToken, err := bp.ParseTokens(openaiReq, req.URL.Path, resp, buf.Bytes())
		metric.InputTokens, metric.OutputTokens, metric.Error = int64(inputToken), int64(outputToken), err.Error()
		updateMetricFn(&metric)
	})

	pc.Resp.SetStatusCode(resp.StatusCode)
	httphelper.RemoveHopByHopHeaders(resp.Header)
	maps.Copy(pc.Resp.HTTPHeader(), resp.Header)
	pc.Resp.SetPayload(body)
	return codeToResult(resp.StatusCode)
}

func (bp *BaseProvider) ParseTokens(openaiReq *protocol.GeneralRequest, path string, resp *http.Response, respBody []byte) (inputToken int, outputToken int, err error) {
	// TODO
	return 0, 0, nil
}
