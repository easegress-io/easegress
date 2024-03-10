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

// Package command provides the commands.
package command

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

func handleRequest(httpMethod string, path string, yamlBody []byte, cmd *cobra.Command) {
	body, err := handleRequestV1(httpMethod, path, yamlBody, cmd)
	if err != nil {
		general.ExitWithError(err)
	}

	if len(body) != 0 {
		general.PrintBody(body)
	}
}

func handleRequestV1(httpMethod string, path string, yamlBody []byte, cmd *cobra.Command) (body []byte, err error) {
	var jsonBody []byte
	if yamlBody != nil {
		var err error
		jsonBody, err = codectool.YAMLToJSON(yamlBody)
		if err != nil {
			return nil, fmt.Errorf("yaml %s to json failed: %v", yamlBody, err)
		}
	}

	url, err := general.MakeURL(path)
	if err != nil {
		return nil, err
	}
	client, err := general.GetHTTPClient()
	if err != nil {
		return nil, err
	}
	resp, body := doRequestV1(httpMethod, url, jsonBody, client, cmd)

	msg := string(body)
	if strings.HasPrefix(url, general.HTTPProtocol) && resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToUpper(msg), "HTTPS") {
		resp, body = doRequestV1(httpMethod, general.HTTPSProtocol+strings.TrimPrefix(url, general.HTTPProtocol), jsonBody, client, cmd)
	}

	if !general.SuccessfulStatusCode(resp.StatusCode) {
		apiErr := &general.APIErr{}
		err := codectool.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		return nil, fmt.Errorf("%d: %s", apiErr.Code, msg)
	}
	return body, nil
}

func doRequestV1(httpMethod string, url string, jsonBody []byte, client *http.Client, cmd *cobra.Command) (*http.Response, []byte) {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(jsonBody))
	if err != nil {
		general.ExitWithError(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		general.ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		general.ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	return resp, body
}
