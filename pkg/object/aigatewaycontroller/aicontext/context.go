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

package aicontext

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

// ResponseType defines the type of response for AI requests.
type ResponseType string

const (
	// ResponseTypeCompletions is used for standard completion requests.
	ResponseTypeCompletions ResponseType = "/v1/completions"
	// ResponseTypeChatCompletions is used for chat completion requests.
	ResponseTypeChatCompletions ResponseType = "/v1/chat/completions"
	// ResponseTypeEmbeddings is used for embedding requests.
	ResponseTypeEmbeddings ResponseType = "/v1/embeddings"
	// ResponseTypeModels is used for model listing requests.
	ResponseTypeModels ResponseType = "/v1/models"
	// ResponseTypeImageGenerations is used for image generation requests.
	ResponseTypeImageGenerations ResponseType = "/v1/images/generations"
)

type ResultError string

const (
	// ResultOk indicates a successful request with no errors.
	// In easegress, it is represented as an empty string.
	ResultOk               ResultError = ""
	ResultInternalError    ResultError = "internalError"
	ResultClientError      ResultError = "clientError"
	ResultServerError      ResultError = "serverError"
	ResultFailureCodeError ResultError = "failureCodeError"
	ResultProviderError    ResultError = "providerError"
	ResultMiddlewareError  ResultError = "middlewareError"
)

func ContextResults() []string {
	return []string{
		string(ResultInternalError),
		string(ResultClientError),
		string(ResultServerError),
		string(ResultFailureCodeError),
		string(ResultProviderError),
		string(ResultMiddlewareError),
	}
}

type (
	// ProviderSpec defines the specification for an AI provider.
	ProviderSpec struct {
		Name         string            `json:"name"`
		ProviderType string            `json:"providerType"`
		BaseURL      string            `json:"baseURL"`
		APIKey       string            `json:"apiKey"`
		Headers      map[string]string `json:"headers,omitempty"`
		// Optional parameters for specific providers, such as Azure.
		Endpoint     string `json:"endpoint,omitempty"`     // It is used for Azure OpenAI.
		DeploymentID string `json:"deploymentID,omitempty"` // It is used for Azure OpenAI.
		APIVersion   string `json:"apiVersion,omitempty"`   // It is used for Azure OpenAI.
	}

	Context struct {
		Ctx      *context.Context
		Provider *ProviderSpec

		// Req is the original request from the user. It's body is in ReqBody.
		Req       *httpprot.Request
		ReqBody   []byte
		ReqInfo   *protocol.GeneralRequest
		OpenAIReq map[string]any
		RespType  ResponseType

		// ParseMetricFn is a function that parses the response body to a metric.
		// If it is sent, it will be called to parse the response body to a metric.
		// Otherwise, default ParseMetricFn will be used.
		ParseMetricFn func(fc *FinishContext) *metricshub.Metric

		resp      *Response
		callBacks []func(fc *FinishContext)

		stop   bool
		result string
	}

	Response struct {
		StatusCode int
		// ContentLength records the length of the associated content. The
		// value -1 indicates that the length is unknown. Unless Request.Method
		// is "HEAD", values >= 0 indicate that the given number of bytes may
		// be read from Body.
		ContentLength int64
		Header        http.Header
		// BodyReader is the response body reader. Only one of BodyReader or BodyBytes should be set.
		BodyReader io.Reader
		// BodyBytes is the response body bytes. Only one of BodyReader or BodyBytes should be set.
		BodyBytes []byte
	}

	FinishContext struct {
		StatusCode int
		Header     http.Header
		// RespBody is the final response body that sent to the user.
		RespBody []byte
		Duration int64
	}
)

func New(ctx *context.Context, provider *ProviderSpec) (*Context, error) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	// request body
	body, err := io.ReadAll(req.GetPayload())
	if err != nil {
		return nil, err
	}

	path := req.URL().Path
	respType := ResponseType("")
	if strings.HasSuffix(path, string(ResponseTypeChatCompletions)) {
		respType = ResponseTypeChatCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeCompletions)) {
		respType = ResponseTypeCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeModels)) {
		respType = ResponseTypeModels
	} else {
		return nil, fmt.Errorf("unsupported request path: %s", path)
	}

	if respType == ResponseTypeModels {
		c := &Context{
			Ctx:       ctx,
			Provider:  provider,
			Req:       req,
			ReqBody:   body,
			OpenAIReq: map[string]any{},
			ReqInfo:   &protocol.GeneralRequest{},
			RespType:  respType,
		}
		return c, nil
	}

	// parse OpenAI request
	openAIReq := make(map[string]interface{})
	err = json.Unmarshal(body, &openAIReq)
	if err != nil {
		return nil, err
	}

	// parse model and stream
	model, _ := openAIReq["model"].(string)
	streamStr := openAIReq["stream"]
	stream := false
	if streamStr != nil {
		stream, _ = strconv.ParseBool(streamStr.(string))
	}
	streamOptions, ok := openAIReq["stream_options"].(protocol.StreamOptions)
	if ok {
		if streamOptions.IncludeUsage == nil {
			// Default to true if not specified
			streamOptions.IncludeUsage = new(bool)
			*streamOptions.IncludeUsage = true
		}
	}

	c := &Context{
		Ctx:       ctx,
		Provider:  provider,
		Req:       req,
		ReqBody:   body,
		OpenAIReq: openAIReq,
		ReqInfo: &protocol.GeneralRequest{
			Model:         model,
			Stream:        stream,
			StreamOptions: streamOptions,
		},
		RespType: respType,
	}
	return c, nil
}

// GetResponse returns the response of the context.
func (c *Context) GetResponse() *Response {
	return c.resp
}

// SetResponse sets the response of the context.
// If you need to close the response body, you should add a callback
// function to the context using AddCallBack method.
func (c *Context) SetResponse(resp *Response) {
	c.resp = resp
}

// AddCallBack adds a callback function to the context.
// The callback will be called when the reponse is sent to the user.
func (c *Context) AddCallBack(cb func(fc *FinishContext)) {
	c.callBacks = append(c.callBacks, cb)
}

// CallBacks returns all callback functions registered in the context.
func (c *Context) Callbacks() []func(fc *FinishContext) {
	return c.callBacks
}

// Stop stops the context execution and sets the result to be returned.
func (c *Context) Stop(result ResultError) {
	c.stop = true
	c.result = string(result)
}

// IsStopped checks if the context execution is stopped.
func (c *Context) IsStopped() bool {
	return c.stop
}

// Result returns the result of the context execution.
func (c *Context) Result() ResultError {
	return ResultError(c.result)
}
