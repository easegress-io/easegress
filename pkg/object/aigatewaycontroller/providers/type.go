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
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

type (
	Provider interface {
		Name() string
		Type() string
		// TODO req is simple http.Request with extra methods.
		// In Easegress, we have multiple namespaces. Every namespace has its own request and response.
		// Here the resp is not the actual response, this resp is used to load data and write to real response when finish.
		// check func (mi *muxInstance) serveHTTP(stdw http.ResponseWriter, stdr *http.Request)
		// and func (mi *muxInstance) sendResponse(ctx *context.Context, stdw http.ResponseWriter) (int, uint64, http.Header)
		// for more details.
		// For non-stream data is simple, just SetPayload([]byte)
		// For stream data, SetPayload(reader) and use a goroutine to read data from backend transfer to openai format and write to reader.
		Handle(ctx *context.Context, req *httpprot.Request, resp *httpprot.Response, updateMetricFn func(*metricshub.Metric)) string

		// HealthCheck checks the health of the provider.
		// It should return nil if the provider is healthy, otherwise it returns an error.
		HealthCheck() error
	}

	// ProviderSpec is the specification for an AI provider.
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
)

const (
	ResultInternalError         = "internalError"
	ResultClientError           = "clientError"
	ResultServerError           = "serverError"
	ResultFailureCode           = "failureCodeError"
	ResultProviderNotFoundError = "providerNotFoundError"
)

func codeToResult(statusCode int) string {
	if statusCode < 400 {
		return ""
	}
	if statusCode < 500 {
		return ResultClientError
	}
	return ResultServerError
}
