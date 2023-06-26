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
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/spf13/cobra"
)

// MakeURL is used to make url for given template.
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

// PrintBody prints the response body in yaml or json format.
func PrintBody(body []byte) {
	if len(body) == 0 {
		return
	}

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

// HandleRequest used in cmd/client/resources. It will return the response body in yaml or json format.
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

// PrintTable prints the table with empty prefix.
func PrintTable(table [][]string) {
	PrintTableWithPrefix(table, "")
}

// PrintTableWithPrefix prints the table with prefix.
func PrintTableWithPrefix(table [][]string, prefix string) {
	w := NewTabWriter()
	defer w.Flush()

	for _, row := range table {
		fmt.Fprintf(w, "%s", prefix)
		for _, col := range row {
			fmt.Fprintf(w, "%s\t", col)
		}
		fmt.Fprintf(w, "\n")
	}
}

// Capitalize capitalizes the first letter of the string.
func Capitalize(str string) string {
	if len(str) == 0 {
		return ""
	}
	return strings.ToUpper(str[0:1]) + str[1:]
}

// DurationMostSignificantUnit returns the most significant unit of the duration.
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

// PrintMapInterface prints the []map[string]interface{} in yaml format. Specials are the keys print in front of other part.
// Use "" in specials to print a blank line.
// For example, if specials is ["name", "kind", "", "filters"]
// then, "name", "kind" will in group one, and "filters" will in group two, others will in group three.
func PrintMapInterface(maps []map[string]interface{}, specials []string) {
	printKV := func(k string, v interface{}) {
		value, err := codectool.MarshalYAML(v)
		if err != nil {
			ExitWithError(err)
		}

		lines := strings.Split(string(value), "\n")
		lines = lines[0 : len(lines)-1]
		if len(lines) == 1 {
			fmt.Printf("%s: %s\n", Capitalize(k), lines[0])
			return
		}
		fmt.Printf("%s:\n", Capitalize(k))
		fmt.Printf("%s\n", strings.Repeat("=", len(k)+1))
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
	}

	print := func(spec map[string]interface{}) {
		type kv struct {
			key   string
			value interface{}
		}

		var kvs []kv
		for k, v := range spec {
			kvs = append(kvs, kv{k, v})
		}
		sort.Slice(kvs, func(i, j int) bool {
			return kvs[i].key < kvs[j].key
		})

		specialFlag := false
		for _, s := range specials {
			if s == "" && specialFlag {
				fmt.Println()
			}
			value, ok := spec[s]
			if ok {
				specialFlag = true
				printKV(s, value)
			} else {
				specialFlag = false
			}
		}

		for _, kv := range kvs {
			if stringtool.StrInSlice(kv.key, specials) {
				continue
			}
			printKV(kv.key, kv.value)
		}
	}

	for i, m := range maps {
		print(m)
		if len(maps) > 1 && i != len(maps)-1 {
			fmt.Print("\n\n---\n\n")
		}
	}
}

// UnmarshalMapInterface unmarshals the body to []map[string]interface{}.
func UnmarshalMapInterface(body []byte, listBody bool) ([]map[string]interface{}, error) {
	if listBody {
		mapInterfaces := []map[string]interface{}{}
		err := codectool.Unmarshal(body, &mapInterfaces)
		return mapInterfaces, err
	}
	mapInterface := map[string]interface{}{}
	err := codectool.Unmarshal(body, &mapInterface)
	return []map[string]interface{}{mapInterface}, err
}

// Example is used to create cobra example.
type Example struct {
	Desc    string
	Command string
}

// CreateExample creates cobra example by using one line example.
func CreateExample(desc, command string) string {
	e := Example{
		Desc:    desc,
		Command: command,
	}
	return CreateMultiExample([]Example{e})
}

// CreateMultiExample creates cobra example by using multiple examples.
func CreateMultiExample(examples []Example) string {
	output := ""
	for i, e := range examples {
		output += fmt.Sprintf("  # %s\n", e.Desc)
		output += fmt.Sprintf("  %s", e.Command)
		if i != len(examples)-1 {
			output += "\n\n"
		}
	}
	return output
}

// GenerateExampleFromChild generates cobra example from child commands.
func GenerateExampleFromChild(cmd *cobra.Command) {
	if len(cmd.Commands()) == 0 {
		return
	}

	example := ""
	for i, c := range cmd.Commands() {
		GenerateExampleFromChild(c)
		if c.Example != "" {
			example += c.Example
		}
		if i != len(cmd.Commands())-1 {
			example += "\n\n"
		}
	}
	cmd.Example = example
}
