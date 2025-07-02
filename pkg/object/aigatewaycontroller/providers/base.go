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

// BaseProvider is a struct that contains the common fields for all AI gateway providers.
// Almost all providers compatible with OpenAI API, so we abstract the common logic.
type BaseProvider struct {
	providerSpec *ProviderSpec
}

var _ Provider = (*BaseProvider)(nil)

func (bp *BaseProvider) Type() string {
	return bp.providerSpec.ProviderType
}

func (bp *BaseProvider) HealthCheck() error {
	// TODO
	return nil
}

func (bp *BaseProvider) Handle(ctx *context.Context, req *httpprot.Request, resp *httpprot.Response, updateMetricFn func(*metricshub.Metric)) string {
	providerContext, err := NewProviderContext(ctx, bp.providerSpec, req, resp)
	if err != nil {
		setEgErrResponse(resp, http.StatusInternalServerError, fmt.Errorf("failed to create provider context: %w", err))
		return ResultInternalError
	}
	request, err := prepareRequest(providerContext, bp.RequestMapper)
	if err != nil {
		setEgErrResponse(resp, http.StatusInternalServerError, err)
		return ResultInternalError
	}

	providerContext.AddCallBack(func(pc *ProviderContext, fc *FinishContext) {
		metric := metricshub.Metric{
			Provider:     pc.Provider.Name,
			ProviderType: pc.Provider.ProviderType,
			Model:        pc.ReqInfo.Model,
			BaseURL:      pc.Provider.BaseURL,
		}
		if fc.Error != nil || fc.Resp == nil || fc.Resp.StatusCode != http.StatusOK {
			metric.Success = false
			metric.Error = ErrMetricInternalServer.Error()
			updateMetricFn(&metric)
			if fc.Error != nil {
				logger.Errorf("provider %s request failed: %v", pc.Provider.Name, fc.Error)
			}
		}
		metric.Success = true
		metric.Duration = fc.Duration

		inputToken, outputToken, err := bp.ParseTokens(pc, fc.Resp, fc.RespBody)
		metric.InputTokens, metric.OutputTokens, metric.Error = int64(inputToken), int64(outputToken), err.Error()
		updateMetricFn(&metric)
	})

	return bp.ProxyRequest(providerContext, request)
}

func (bp *BaseProvider) RequestMapper(pc *ProviderContext) (string, []byte, error) {
	return string(pc.RespType), pc.ReqBody, nil
}

func (bp *BaseProvider) ProxyRequest(pc *ProviderContext, req *http.Request) string {
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setEgErrResponse(pc.Resp, http.StatusInternalServerError, err)

		fc := &FinishContext{
			Error: err,
		}
		pc.Finish(fc)
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
		fc := &FinishContext{
			Resp:     resp,
			RespBody: buf.Bytes(),
			Duration: duration,
			Error:    nil,
		}
		pc.Finish(fc)
	})

	pc.Resp.SetStatusCode(resp.StatusCode)
	httphelper.RemoveHopByHopHeaders(resp.Header)
	maps.Copy(pc.Resp.HTTPHeader(), resp.Header)
	pc.Resp.SetPayload(body)
	return codeToResult(resp.StatusCode)
}

func (bp *BaseProvider) ParseTokens(pc *ProviderContext, resp *http.Response, respBody []byte) (inputToken int, outputToken int, err error) {
	openaiReq := pc.ReqInfo
	if resp.StatusCode != http.StatusOK {
		respErr := &protocol.ErrorResponse{}
		err := json.Unmarshal(respBody, &respErr)
		if err != nil {
			logger.Errorf("failed to unmarshal resp body, %v", err)
			return 0, 0, ErrMetricUnmarshalResponse
		}
		return 0, 0, errors.New(respErr.Error.Type)
	}

	switch pc.RespType {
	case ResponseTypeCompletions:
		return parseCompletions(openaiReq, respBody)
	case ResponseTypeChatCompletions:
		return parseChatCompletions(openaiReq, respBody)
	case ResponseTypeEmbeddings:
		return parseEmbeddings(respBody)
	case ResponseTypeImageGenerations:
		return parseImageGenerations(respBody)
	case ResponseTypeModels:
		return 0, 0, nil
	default:
		logger.Errorf("unsupported resp type %s", pc.RespType)
		return 0, 0, ErrMetricInternalServer
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
			return 0, 0, ErrMetricUnmarshalResponse
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
		return 0, 0, ErrMetricUnmarshalResponse
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
			return 0, 0, ErrMetricUnmarshalResponse
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
		return 0, 0, ErrMetricUnmarshalResponse
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}

func parseEmbeddings(respBody []byte) (inputToken int, outputToken int, err error) {
	resp := &protocol.EmbeddingResponse{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, ErrMetricUnmarshalResponse
	}
	return resp.Usage.PromptTokens, resp.Usage.TotalTokens, nil
}

func parseImageGenerations(respBody []byte) (inputToken int, outputToken int, err error) {
	resp := &protocol.ImageResponse{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, ErrMetricUnmarshalResponse
	}
	return resp.Usage.InputTokens, resp.Usage.OutputTokens, nil
}

func getLastChunkFromOpenAIStream(body []byte) ([]byte, error) {
	bodyString := strings.TrimSpace(string(body))
	chunks := strings.Split(bodyString, "\n\n")
	l := len(chunks)
	if l <= 1 {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, ErrMetricUnmarshalResponse
	}
	if chunks[l-1] != "data: [DONE]" {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, ErrMetricUnmarshalResponse
	}
	return []byte(strings.TrimPrefix(chunks[l-2], "data: ")), nil
}
