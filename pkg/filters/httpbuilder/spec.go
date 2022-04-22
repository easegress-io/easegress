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

package httpbuilder

type (
	// BodySpec describes how to build the body of the request.
	BodySpec struct {
		Requests  []*ReqRespBody `yaml:"requests" jsonschema:"omitempty"`
		Responses []*ReqRespBody `yaml:"responses" jsonschema:"omitempty"`
		Body      string         `yaml:"body" jsonschema:"omitempty"`
	}

	// ReqRespBody describes the request body or response body used to create new request.
	ReqRespBody struct {
		ID     string `yaml:"id" jsonschema:"required"`
		UseMap bool   `yaml:"useMap" jsonschema:"omitempty"`
	}

	// Header defines HTTP header template.
	Header struct {
		Key   string `yaml:"key"`
		Value string `yaml:"value"`
	}

	// StatusCode is status code.
	StatusCode struct {
		CopyResponseID string `yaml:"copyResponseID" jsonschema:"omitempty"`
		Code           int    `yaml:"code" jsonschema:"omitempty"`
	}
)
