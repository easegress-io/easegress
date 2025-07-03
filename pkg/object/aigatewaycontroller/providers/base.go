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
	"net/url"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
)

// BaseProvider is a struct that contains the common fields for all AI gateway providers.
// Almost all providers compatible with OpenAI API, so we abstract the common logic.
type BaseProvider struct {
	providerSpec *aicontext.ProviderSpec
}

var _ Provider = (*BaseProvider)(nil)

func (bp *BaseProvider) Type() string {
	return bp.providerSpec.ProviderType
}

func (bp *BaseProvider) Name() string {
	return bp.providerSpec.Name
}

func (bp *BaseProvider) Spec() *aicontext.ProviderSpec {
	return bp.providerSpec
}

func (bp *BaseProvider) HealthCheck() error {
	checkURL, err := url.JoinPath(bp.providerSpec.BaseURL, string(aicontext.ResponseTypeModels))
	if err != nil {
		return fmt.Errorf("failed to join health check URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed, status code: %d", resp.StatusCode)
	}
	return nil
}

func (bp *BaseProvider) Handle(ctx *aicontext.Context) {
	request, err := prepareRequest(ctx, bp.RequestMapper)
	if err != nil {
		logger.Errorf("failed to prepare request for provider %s: %v", bp.providerSpec.Name, err)
		setErrResponse(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ParseMetricFn = func(fc *aicontext.FinishContext) *metricshub.Metric {
		metric := &metricshub.Metric{
			Provider:     ctx.Provider.Name,
			ProviderType: ctx.Provider.ProviderType,
			Model:        ctx.ReqInfo.Model,
			BaseURL:      ctx.Provider.BaseURL,
			ResponseType: string(ctx.RespType),
		}
		if fc.StatusCode != http.StatusOK {
			metric.Success = false
			metric.Error = metricshub.MetricInternalError
			return metric
		}
		metric.Success = true
		metric.Duration = fc.Duration
		inputToken, outputToken, err := bp.ParseTokens(ctx, fc, fc.RespBody)
		metric.InputTokens, metric.OutputTokens, metric.Error = int64(inputToken), int64(outputToken), err
		return metric
	}

	bp.ProxyRequest(ctx, request)
}

func (bp *BaseProvider) RequestMapper(pc *aicontext.Context) (string, []byte, error) {
	return string(pc.RespType), pc.ReqBody, nil
}

func (bp *BaseProvider) ProxyRequest(ctx *aicontext.Context, req *http.Request) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setErrResponse(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.AddCallBack(func(*aicontext.FinishContext) {
		resp.Body.Close()
	})

	ctx.SetResponse(&aicontext.Response{
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Header:        resp.Header,
		BodyReader:    resp.Body,
	})
	if resp.StatusCode != http.StatusOK {
		ctx.Stop(aicontext.ResultProviderError)
	}
}

func (bp *BaseProvider) ParseTokens(ctx *aicontext.Context, fc *aicontext.FinishContext, respBody []byte) (inputToken int, outputToken int, err metricshub.MetricError) {
	openaiReq := ctx.ReqInfo
	if fc.StatusCode != http.StatusOK {
		respErr := &protocol.ErrorResponse{}
		err := json.Unmarshal(respBody, &respErr)
		if err != nil {
			logger.Errorf("failed to unmarshal resp body, %v", err)
			return 0, 0, metricshub.MetricMarshalError
		}
		return 0, 0, metricshub.MetricError(respErr.Error.Type)
	}

	switch ctx.RespType {
	case aicontext.ResponseTypeCompletions:
		return parseCompletions(openaiReq, fc.RespBody)
	case aicontext.ResponseTypeChatCompletions:
		return parseChatCompletions(openaiReq, fc.RespBody)
	case aicontext.ResponseTypeEmbeddings:
		return parseEmbeddings(fc.RespBody)
	case aicontext.ResponseTypeImageGenerations:
		return parseImageGenerations(fc.RespBody)
	case aicontext.ResponseTypeModels:
		return 0, 0, metricshub.MetricNoError
	default:
		logger.Errorf("unsupported resp type %s", ctx.RespType)
		return 0, 0, metricshub.MetricNoError
	}
}

func parseCompletions(openaiReq *protocol.GeneralRequest, respBody []byte) (inputToken int, outputToken int, e metricshub.MetricError) {
	if openaiReq.Stream {
		chunk, e := getLastChunkFromOpenAIStream(respBody)
		if e != "" {
			return 0, 0, e
		}
		resp := &protocol.CompletionChunk{}
		err := json.Unmarshal(chunk, &resp)
		if err != nil {
			logger.Errorf("failed to unmarshal resp %s, %v", string(chunk), err)
			return 0, 0, metricshub.MetricMarshalError
		}
		if resp.Usage != nil {
			return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, ""
		}
		return 0, 0, ""
	}
	resp := &protocol.Completion{}
	err := json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, metricshub.MetricMarshalError
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, ""
}

func parseChatCompletions(openaiReq *protocol.GeneralRequest, respBody []byte) (inputToken int, outputToken int, e metricshub.MetricError) {
	if openaiReq.Stream {
		chunk, e := getLastChunkFromOpenAIStream(respBody)
		if e != "" {
			return 0, 0, e
		}
		resp := &protocol.ChatCompletionChunk{}
		err := json.Unmarshal(chunk, &resp)
		if err != nil {
			logger.Errorf("failed to unmarshal resp %s, %v", string(chunk), err)
			return 0, 0, metricshub.MetricMarshalError
		}
		if resp.Usage != nil {
			return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, ""
		}
		return 0, 0, ""
	}
	resp := &protocol.ChatCompletion{}
	err := json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, metricshub.MetricMarshalError
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, ""
}

func parseEmbeddings(respBody []byte) (inputToken int, outputToken int, e metricshub.MetricError) {
	resp := &protocol.EmbeddingResponse{}
	err := json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, metricshub.MetricMarshalError
	}
	return resp.Usage.PromptTokens, resp.Usage.TotalTokens, ""
}

func parseImageGenerations(respBody []byte) (inputToken int, outputToken int, e metricshub.MetricError) {
	resp := &protocol.ImageResponse{}
	err := json.Unmarshal(respBody, &resp)
	if err != nil {
		logger.Errorf("failed to unmarshal resp %s, %v", string(respBody), err)
		return 0, 0, metricshub.MetricMarshalError
	}
	return resp.Usage.InputTokens, resp.Usage.OutputTokens, ""
}

func getLastChunkFromOpenAIStream(body []byte) ([]byte, metricshub.MetricError) {
	bodyString := strings.TrimSpace(string(body))
	chunks := strings.Split(bodyString, "\n\n")
	l := len(chunks)
	if l <= 1 {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, metricshub.MetricMarshalError
	}
	if chunks[l-1] != "data: [DONE]" {
		logger.Errorf("failed to get last chunk from response %s", string(body))
		return nil, metricshub.MetricMarshalError
	}
	return []byte(strings.TrimPrefix(chunks[l-2], "data: ")), ""
}
