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
	"fmt"
	"maps"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/httphelper"
)

type RequestMapper func(pc *aicontext.Context) (path string, newBody []byte, err error)

func prepareRequest(pc *aicontext.Context, mapper RequestMapper) (request *http.Request, err error) {
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

	// req.Header.Set("Accept-Encoding", "identity")

	return req, err
}

func setErrResponse(ctx *aicontext.Context, code int, err error) {
	errMsg := protocol.NewError(code, err.Error())
	data, _ := codectool.MarshalJSON(errMsg)
	ctx.SetResponse(&aicontext.Response{
		StatusCode:    code,
		ContentLength: int64(len(data)),
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		BodyBytes:     data,
	})
	ctx.Stop(aicontext.ResultInternalError)
}
