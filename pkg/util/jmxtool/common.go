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

// Package jmxtool provides some tools for jmx
package jmxtool

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

// JSONToKVMap converts JSON string to key value pairs
func JSONToKVMap(jsonStr string) (map[string]string, error) {
	m := map[string]interface{}{}
	err := codectool.Unmarshal([]byte(jsonStr), &m)
	if err != nil {
		return nil, err
	}
	kvs := extractKVs("", m)
	resultMap := map[string]string{}
	length := len(kvs)
	for i := 0; i < length; i++ {
		tmp := kvs[i]
		for k, v := range tmp {
			resultMap[k] = v
		}

	}
	return resultMap, nil
}

func extractKVs(prefix string, obj interface{}) []map[string]string {
	var rst []map[string]string

	switch o := obj.(type) {
	case map[string]interface{}:
		for k, v := range o {
			current := k
			rst = append(rst, extractKVs(join(prefix, current), v)...)
		}

	case []interface{}:
		length := len(o)
		for i := 0; i < length; i++ {
			rst = append(rst, extractKVs(join(prefix, strconv.Itoa(i)), o[i])...)
		}
	case bool:
		rst = append(rst, map[string]string{prefix: strconv.FormatBool(o)})
	case int:
		rst = append(rst, map[string]string{prefix: strconv.Itoa(o)})
	case float64:
		rst = append(rst, map[string]string{prefix: strconv.FormatFloat(o, 'f', 0, 64)})
	default:
		if obj == nil {
			rst = append(rst, map[string]string{prefix: ""})
		} else {
			rst = append(rst, map[string]string{prefix: obj.(string)})
		}
	}
	return rst
}

func join(prefix string, current string) string {
	if prefix == "" {
		return current
	}
	return strings.Join([]string{prefix, current}, ".")
}

// APIErr is the error struct for API
type APIErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func handleRequest(httpMethod string, url string, reqBody []byte) ([]byte, error) {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if successfulStatusCode(resp.StatusCode) {
		return body, nil
	}

	msg := string(body)
	apiErr := &APIErr{}
	err = codectool.Unmarshal(body, apiErr)
	if err == nil {
		msg = apiErr.Message
	}

	return nil, fmt.Errorf("Request failed: Code: %d, Msg: %s ", resp.StatusCode, msg)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}
