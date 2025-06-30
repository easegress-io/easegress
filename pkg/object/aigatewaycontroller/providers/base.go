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
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/httphelper"
	"github.com/megaease/easegress/v2/pkg/util/readers"
)

type ResponseType string

const (
	ResponseTypeCompletions      ResponseType = "/v1/completions"
	ResponseTypeChatCompletions  ResponseType = "/v1/chat/completions"
	ResponseTypeEmbeddings       ResponseType = "/v1/embeddings"
	ResponseTypeModels           ResponseType = "/v1/models"
	ResponseTypeImageGenerations ResponseType = "/v1/images/generations"
)

// pathToResponseType maps the request path to the corresponding response type.
// some of provider support OpenAI API with differne path.
var pathToResponseType = map[string]ResponseType{
	"/v1/completions":      ResponseTypeCompletions,
	"/v1/chat/completions": ResponseTypeChatCompletions,
}

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
	if strings.HasSuffix(path, string(ResponseTypeChatCompletions)) {
		path = string(ResponseTypeChatCompletions)
	} else if strings.HasSuffix(path, string(ResponseTypeCompletions)) {
		path = string(ResponseTypeCompletions)
	} else if strings.HasSuffix(path, string(ResponseTypeEmbeddings)) {
		path = string(ResponseTypeEmbeddings)
	} else if strings.HasSuffix(path, string(ResponseTypeModels)) {
		path = string(ResponseTypeModels)
	} else if strings.HasSuffix(path, string(ResponseTypeImageGenerations)) {
		path = string(ResponseTypeImageGenerations)
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
	if resp.StatusCode != http.StatusOK {
		respErr := &protocol.ErrorResponse{}
		err := json.Unmarshal(respBody, &respErr)
		if err != nil {
			logger.Errorf("failed to unmarshal resp body, %v", err)
			return 0, 0, ErrUnmarshalResponse
		}
		return 0, 0, errors.New(respErr.Error.Type)
	}
	respType, ok := pathToResponseType[path]
	if !ok {
		logger.Errorf("unknow path %s to parse response", path)
		return 0, 0, ErrInternalServer
	}

	switch respType {
	case ResponseTypeCompletions:
		return parseCompletions(openaiReq, respBody)
	case ResponseTypeChatCompletions:
		return parseChatCompletions(openaiReq, respBody)
	// TODO:
	case ResponseTypeEmbeddings:
		return 0, 0, nil
	case ResponseTypeModels:
		return 0, 0, nil
	case ResponseTypeImageGenerations:
		return 0, 0, nil
	default:
		logger.Errorf("unsupported resp type %s", respType)
		return 0, 0, ErrInternalServer
	}
}

func parseCompletions(openaiReq *protocol.GeneralRequest, respBody []byte) (inputToken int, outputToken int, err error) {
	if openaiReq.Stream {
		chunk, err := getLastChunkFromOpenAIStream(respBody)
		if err != nil {
			return 0, 0, err
		}
		resp := &protocol.CompletionChunk{}
		err = json.Unmarshal(chunk, &resp)
		if err != nil {
			logger.Errorf("failed to unmarshal resp %s, %v", string(chunk), err)
			return 0, 0, ErrUnmarshalResponse
		}
		if resp.Usage != nil {
			return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
		}
		return 0, 0, nil
	}
	resp := &protocol.Completion{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, ErrUnmarshalResponse
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}

func parseChatCompletions(openaiReq *protocol.GeneralRequest, respBody []byte) (inputToken int, outputToken int, err error) {
	if openaiReq.Stream {
		chunk, err := getLastChunkFromOpenAIStream(respBody)
		if err != nil {
			return 0, 0, err
		}
		resp := &protocol.ChatCompletionChunk{}
		err = json.Unmarshal(chunk, &resp)
		if err != nil {
			logger.Errorf("failed to unmarshal resp %s, %v", string(chunk), err)
			return 0, 0, ErrUnmarshalResponse
		}
		if resp.Usage != nil {
			return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
		}
		return 0, 0, nil
	}
	resp := &protocol.ChatCompletion{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, ErrUnmarshalResponse
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}

func getLastChunkFromOpenAIStream(body []byte) ([]byte, error) {
	bodyString := strings.TrimSpace(string(body))
	chunks := strings.Split(bodyString, "\n\n")
	l := len(chunks)
	if l <= 1 {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, ErrUnmarshalResponse
	}
	if chunks[l-1] != "data: [DONE]" {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, ErrUnmarshalResponse
	}
	return []byte(strings.TrimPrefix(chunks[l-2], "data: ")), nil
}
