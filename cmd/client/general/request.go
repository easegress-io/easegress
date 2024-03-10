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

// Package general provides the general utilities for the client.
package general

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// MakeURL is used to make URL for given path.
func MakeURL(path string) (string, error) {
	// server priority: flag > config > default
	server := defaultServer
	config, err := GetCurrentConfig()
	if err != nil {
		return "", err
	}
	if config != nil && config.GetServer() != "" {
		server = config.GetServer()
	}
	if CmdGlobalFlags.Server != "" {
		server = CmdGlobalFlags.Server
	}

	// protocol priority: https > http
	p := HTTPProtocol
	if CmdGlobalFlags.ForceTLS || config != nil && config.UseHTTPS() {
		p = HTTPSProtocol
	}
	if strings.HasPrefix(server, HTTPSProtocol) {
		return server + path, nil
	}
	return p + strings.TrimPrefix(server, HTTPProtocol) + path, nil
}

// MakePath is used to make path for given template.
func MakePath(urlTemplate string, a ...interface{}) string {
	return fmt.Sprintf(urlTemplate, a...)
}

// GetHTTPClient is used to get HTTP client.
func GetHTTPClient() (*http.Client, error) {
	config, err := GetCurrentConfig()
	if err != nil {
		return nil, err
	}
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: CmdGlobalFlags.InsecureSkipVerify},
	}

	if config != nil {
		// set client certificate
		if config.GetClientCertificateData() != nil || config.GetClientKeyData() != nil {
			cert, err := tls.X509KeyPair(config.GetClientCertificateData(), config.GetClientKeyData())
			if err != nil {
				return nil, err
			}
			tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
		}
		if config.GetClientCertificate() != "" || config.GetClientKey() != "" {
			cert, err := tls.LoadX509KeyPair(config.GetClientCertificate(), config.GetClientKey())
			if err != nil {
				return nil, err
			}
			tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
		}

		// set CA certificate
		if config.GetCertificateAuthorityData() != nil {
			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(config.GetCertificateAuthorityData()); !ok {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}
			tr.TLSClientConfig.RootCAs = caCertPool
		}
		if config.GetCertificateAuthority() != "" {
			caCertPool := x509.NewCertPool()
			caCert, err := os.ReadFile(config.GetCertificateAuthority())
			if err != nil {
				return nil, err
			}
			if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}
			tr.TLSClientConfig.RootCAs = caCertPool
		}
	}

	client := &http.Client{
		Transport: &tr,
	}
	return client, nil
}

// SuccessfulStatusCode returns true if the status code is successful.
func SuccessfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

func HandleReqWithStreamResp(httpMethod string, path string, yamlBody []byte) (io.ReadCloser, error) {
	var jsonBody []byte
	if yamlBody != nil {
		var err error
		jsonBody, err = codectool.YAMLToJSON(yamlBody)
		if err != nil {
			return nil, fmt.Errorf("yaml %s to json failed: %v", yamlBody, err)
		}
	}

	url, err := MakeURL(path)
	if err != nil {
		return nil, err
	}
	client, err := GetHTTPClient()
	if err != nil {
		return nil, err
	}
	resp, err := doRequest(httpMethod, url, jsonBody, client)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(url, HTTPProtocol) && resp.StatusCode == http.StatusBadRequest {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body failed: %v", err)
		}

		// https://github.com/golang/go/blob/release-branch.go1.21/src/net/http/server.go#L1892-L1899
		if strings.Contains(string(body), "Client sent an HTTP request to an HTTPS server") {
			resp, err = doRequest(httpMethod, HTTPSProtocol+strings.TrimPrefix(url, HTTPProtocol), jsonBody, client)
			if err != nil {
				return nil, err
			}
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	if !SuccessfulStatusCode(resp.StatusCode) {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response body failed: %v", err)
		}

		msg := string(body)
		apiErr := &APIErr{}
		err = codectool.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		return nil, fmt.Errorf("%d: %s", apiErr.Code, msg)
	}
	return resp.Body, nil
}

// HandleRequest used in cmd/client/resources. It will return the response body in yaml or json format.
func HandleRequest(httpMethod string, path string, yamlBody []byte) (body []byte, err error) {
	var jsonBody []byte
	if yamlBody != nil {
		var err error
		jsonBody, err = codectool.YAMLToJSON(yamlBody)
		if err != nil {
			return nil, fmt.Errorf("yaml %s to json failed: %v", yamlBody, err)
		}
	}

	url, err := MakeURL(path)
	if err != nil {
		return nil, err
	}
	client, err := GetHTTPClient()
	if err != nil {
		return nil, err
	}
	resp, body, err := doRequestWithBody(httpMethod, url, jsonBody, client)
	if err != nil {
		return nil, err
	}

	msg := string(body)
	// https://github.com/golang/go/blob/release-branch.go1.21/src/net/http/server.go#L1892-L1899
	if strings.HasPrefix(url, HTTPProtocol) && resp.StatusCode == http.StatusBadRequest && strings.Contains(msg, "Client sent an HTTP request to an HTTPS server") {
		resp, body, err = doRequestWithBody(httpMethod, HTTPSProtocol+strings.TrimPrefix(url, HTTPProtocol), jsonBody, client)
		if err != nil {
			return nil, err
		}
	}

	if !SuccessfulStatusCode(resp.StatusCode) {
		apiErr := &APIErr{}
		err := codectool.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		return nil, fmt.Errorf("%d: %s", apiErr.Code, msg)
	}
	return body, nil
}

func doRequest(httpMethod string, url string, jsonBody []byte, client *http.Client) (*http.Response, error) {
	config, err := GetCurrentConfig()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(jsonBody))
	if config != nil && config.GetUsername() != "" {
		req.SetBasicAuth(config.GetUsername(), config.GetPassword())
	}
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func doRequestWithBody(httpMethod string, url string, jsonBody []byte, client *http.Client) (*http.Response, []byte, error) {
	resp, err := doRequest(httpMethod, url, jsonBody, client)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

// ArgInfo is used to store the information of arguments.
type ArgInfo struct {
	Resource string
	Name     string
	Other    string
}

// ContainResource returns true if the arguments contain resource.
func (a *ArgInfo) ContainResource() bool {
	return a.Resource != ""
}

// ContainName returns true if the arguments contain name.
func (a *ArgInfo) ContainName() bool {
	return a.Name != ""
}

// ContainOther returns true if the arguments contain other.
func (a *ArgInfo) ContainOther() bool {
	return a.Other != ""
}

// ParseArgs parses the arguments and returns the ArgInfo.
func ParseArgs(args []string) *ArgInfo {
	if len(args) == 0 {
		return nil
	}
	argInfo := &ArgInfo{}
	argInfo.Resource = args[0]
	if len(args) > 1 {
		argInfo.Name = args[1]
	}
	if len(args) > 2 {
		argInfo.Other = args[2]
	}
	return argInfo
}

// InAPIResource returns true if the arg is in the api resource.
func InAPIResource(arg string, r *api.APIResource) bool {
	return arg == r.Name || arg == r.Kind || stringtool.StrInSlice(arg, r.Aliases)
}
