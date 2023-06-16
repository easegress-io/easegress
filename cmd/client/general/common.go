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

// Package command implements commands of Easegress client.
package general

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

type (
	// GlobalFlags is the global flags for the whole client.
	GlobalFlags struct {
		Server             string
		ForceTLS           bool
		InsecureSkipVerify bool
		OutputFormat       string
	}

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

func (g *GlobalFlags) DefaultFormat() bool {
	return g.OutputFormat == DefaultFormat
}

// CmdGlobalFlags is the singleton of GlobalFlags.
var CmdGlobalFlags GlobalFlags

const (
	ApiURL = "/apis/v2"

	HealthURL = ApiURL + "/healthz"

	MembersURL = ApiURL + "/status/members"
	MemberURL  = ApiURL + "/status/members/%s"

	ObjectKindsURL = ApiURL + "/object-kinds"
	ObjectsURL     = ApiURL + "/objects"
	ObjectURL      = ApiURL + "/objects/%s"

	StatusObjectURL  = ApiURL + "/status/objects/%s"
	StatusObjectsURL = ApiURL + "/status/objects"

	WasmCodeURL = ApiURL + "/wasm/code"
	WasmDataURL = ApiURL + "/wasm/data/%s/%s"

	CustomDataKindURL     = ApiURL + "/customdatakinds"
	CustomDataKindItemURL = ApiURL + "/customdatakinds/%s"
	CustomDataURL         = ApiURL + "/customdata/%s"
	CustomDataItemURL     = ApiURL + "/customdata/%s/%s"

	ProfileURL      = ApiURL + "/profile"
	ProfileStartURL = ApiURL + "/profile/start/%s"
	ProfileStopURL  = ApiURL + "/profile/stop"

	// HTTPProtocol is prefix for HTTP protocol
	HTTPProtocol = "http://"
	// HTTPSProtocol is prefix for HTTPS protocol
	HTTPSProtocol = "https://"
)

func MakeURL(urlTemplate string, a ...interface{}) string {
	return CmdGlobalFlags.Server + fmt.Sprintf(urlTemplate, a...)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

// HandleRequestV1 used in cmd/client/command. It will print the response body in yaml or json format.
func HandleRequestV1(httpMethod string, url string, yamlBody []byte, cmd *cobra.Command) {
	body, err := HandleRequest(httpMethod, url, yamlBody, cmd)
	if err != nil {
		ExitWithError(err)
	}

	if len(body) != 0 {
		PrintBody(body)
	}
}

func PrintBody(body []byte) {
	var output []byte
	switch CmdGlobalFlags.OutputFormat {
	case JsonFormat:
		var prettyJSON bytes.Buffer
		err := json.Indent(&prettyJSON, body, "", "  ")
		if err != nil {
			output = body
		} else {
			output = prettyJSON.Bytes()
		}
	default:
		var err error
		output, err = codectool.JSONToYAML(body)
		if err != nil {
			ExitWithErrorf("json %s to yaml failed: %v", body, err)
		}
	}

	fmt.Printf("%s", output)
}

func HandleRequest(httpMethod string, url string, yamlBody []byte, cmd *cobra.Command) (body []byte, err error) {
	var jsonBody []byte
	if yamlBody != nil {
		var err error
		jsonBody, err = codectool.YAMLToJSON(yamlBody)
		if err != nil {
			return nil, fmt.Errorf("yaml %s to json failed: %v", yamlBody, err)
		}
	}

	p := HTTPProtocol
	if CmdGlobalFlags.ForceTLS {
		p = HTTPSProtocol
	}
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: CmdGlobalFlags.InsecureSkipVerify},
	}
	client := &http.Client{Transport: &tr}
	resp, body := doRequest(httpMethod, p+url, jsonBody, client, cmd)

	msg := string(body)
	if p == HTTPProtocol && resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToUpper(msg), "HTTPS") {
		resp, body = doRequest(httpMethod, HTTPSProtocol+url, jsonBody, client, cmd)
	}

	if !successfulStatusCode(resp.StatusCode) {
		apiErr := &APIErr{}
		err := codectool.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		return nil, fmt.Errorf("%d: %s", apiErr.Code, msg)
	}
	return body, nil
}

func doRequest(httpMethod string, url string, jsonBody []byte, client *http.Client, cmd *cobra.Command) (*http.Response, []byte) {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(jsonBody))
	if err != nil {
		ExitWithError(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	return resp, body
}

// NewTabWriter returns a tabwriter.Writer. Remember to call Flush() after using it.
func NewTabWriter() *tabwriter.Writer {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	return w
}

func PrintTable(table [][]string) {
	w := NewTabWriter()
	defer w.Flush()

	for _, row := range table {
		for _, col := range row {
			fmt.Fprintf(w, "%s\t", col)
		}
		fmt.Fprintf(w, "\n")
	}
}

func DurationMostSignificantUnit(d time.Duration) string {
	total := float64(d)

	day := total / float64(time.Hour*24)
	if day >= 1 {
		return fmt.Sprintf("%.0fd", day)
	}

	hour := total / float64(time.Hour)
	if hour >= 1 {
		return fmt.Sprintf("%.0fh", hour)
	}

	min := total / float64(time.Minute)
	if min >= 1 {
		return fmt.Sprintf("%.0fm", min)
	}

	sec := total / float64(time.Second)
	if sec >= 1 {
		return fmt.Sprintf("%.0fs", sec)
	}

	return "0s"
}
