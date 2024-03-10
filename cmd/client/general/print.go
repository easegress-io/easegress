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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/spf13/cobra"
)

// PrintBody prints the response body in yaml or json format.
func PrintBody(body []byte) {
	if len(body) == 0 {
		return
	}

	var output []byte
	switch CmdGlobalFlags.OutputFormat {
	case JSONFormat:
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

func lowerFirstLetter(str string) string {
	if len(str) == 0 {
		return ""
	}
	return strings.ToLower(str[0:1]) + str[1:]
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
func PrintMapInterface(maps []map[string]interface{}, fronts []string, backs []string) {
	cutLine := func(line string) string {
		if CmdGlobalFlags.Verbose || len(line) < 50 {
			return line
		}
		return line[0:50] + "..."
	}

	printKV := func(k string, v interface{}) {
		value, err := codectool.MarshalYAML(v)
		if err != nil {
			ExitWithError(err)
		}

		lines := strings.Split(string(value), "\n")
		lines = lines[0 : len(lines)-1]
		if len(lines) == 1 {
			fmt.Printf("%s: %s\n", Capitalize(k), cutLine(lines[0]))
			return
		}
		fmt.Printf("%s:\n", Capitalize(k))
		fmt.Printf("%s\n", strings.Repeat("=", len(k)+1))
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
	}

	printFn := func(spec map[string]interface{}) {
		spec = func(s map[string]interface{}) map[string]interface{} {
			res := map[string]interface{}{}
			for k, v := range s {
				res[lowerFirstLetter(k)] = v
			}
			return res
		}(spec)

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

		frontFlag := false
		for _, f := range fronts {
			if f == "" && frontFlag {
				fmt.Println()
			}
			value, ok := spec[f]
			if ok {
				frontFlag = true
				printKV(f, value)
			} else {
				frontFlag = false
			}
		}

		for _, kv := range kvs {
			if stringtool.StrInSlice(kv.key, fronts) {
				continue
			}
			if stringtool.StrInSlice(kv.key, backs) {
				continue
			}
			printKV(kv.key, kv.value)
		}

		backFlag := false
		for _, b := range backs {
			value, ok := spec[b]
			if ok {
				if !backFlag {
					fmt.Println()
					backFlag = true
				}
				printKV(b, value)
			}
		}
	}

	for i, m := range maps {
		printFn(m)
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

// CreateMultiLineExample creates cobra example by using multiple lines.
func CreateMultiLineExample(example string) string {
	lines := strings.Split(example, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

// GenerateExampleFromChild generates cobra example from child commands.
func GenerateExampleFromChild(cmd *cobra.Command) {
	if len(cmd.Commands()) == 0 {
		return
	}
	if cmd.Example != "" {
		return
	}

	example := ""
	for i, c := range cmd.Commands() {
		GenerateExampleFromChild(c)
		if c.Example != "" {
			example += c.Example
		}
		if i != len(cmd.Commands())-1 && c.Example != "" {
			example += "\n\n"
		}
	}
	cmd.Example = example
}

// Filter filters the array by using filter function.
func Filter[T any](array []T, filter func(value T) bool) []T {
	var result []T
	for _, v := range array {
		if filter(v) {
			result = append(result, v)
		}
	}
	return result
}

// Find finds the first element in the array by using find function.
func Find[T any](array []T, find func(value T) bool) (*T, bool) {
	for _, v := range array {
		if find(v) {
			return &v, true
		}
	}
	return nil, false
}

// Map maps the array by using map function.
func Map[T1 any, T2 any](array []T1, mapF func(value T1) T2) []T2 {
	var result []T2
	for _, v := range array {
		result = append(result, mapF(v))
	}
	return result
}
