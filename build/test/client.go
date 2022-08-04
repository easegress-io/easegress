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

package test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func makeURL(template string, a ...interface{}) string {
	return "http://127.0.0.1:12381/apis/v2" + fmt.Sprintf(template, a...)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

func intentString(str string) string {
	return "\n\t\t" + strings.ReplaceAll(str, "\n", "\n\t\t")
}

func handleRequest(t *testing.T, method string, url string, reader io.Reader) *http.Response {
	req, err := http.NewRequest(method, url, reader)
	require.Nil(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	return resp
}

const (
	objectsURL = "/objects"
	objectURL  = "/objects/%s"
)

func createObject(t *testing.T, yamlFile string) (ok bool, msg string) {
	resp := handleRequest(t, http.MethodPost, makeURL(objectsURL), strings.NewReader(yamlFile))
	defer resp.Body.Close()

	ok = successfulStatusCode(resp.StatusCode)
	if !ok {
		data, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		msg = fmt.Sprintf("create object\n %v\nfailed, %v", intentString(yamlFile), intentString(string(data)))
	}
	return
}

func updateObject(t *testing.T, name string, yamlFile string) (ok bool, msg string) {
	resp := handleRequest(t, http.MethodPut, makeURL(objectURL, name), strings.NewReader(yamlFile))
	defer resp.Body.Close()

	ok = successfulStatusCode(resp.StatusCode)
	if !ok {
		data, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		msg = fmt.Sprintf("update object %v\n %v\nfailed, %v", name, intentString(yamlFile), intentString(string(data)))
	}
	return
}

func deleteObject(t *testing.T, name string) (ok bool, msg string) {
	resp := handleRequest(t, http.MethodDelete, makeURL(objectURL, name), nil)
	defer resp.Body.Close()

	ok = successfulStatusCode(resp.StatusCode)
	if !ok {
		data, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		msg = fmt.Sprintf("delete object %v failed, %v", name, intentString(string(data)))
	}
	return ok, msg
}

func listObject(t *testing.T) (ok bool, msg string) {
	resp := handleRequest(t, http.MethodGet, makeURL(objectsURL), nil)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.Nil(t, err)
	return successfulStatusCode(resp.StatusCode), string(data)
}
