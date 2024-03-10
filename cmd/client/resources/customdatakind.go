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

// Package resources provides the resources utilities for the client.
package resources

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/cluster/customdata"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// CustomDataKind is CustomDataKind resource.
func CustomDataKind() *api.APIResource {
	return &api.APIResource{
		Kind:    "CustomDataKind",
		Name:    "customdatakind",
		Aliases: []string{"customdatakinds", "cdk"},
	}
}

// DescribeCustomDataKind describes the custom data kind.
func DescribeCustomDataKind(cmd *cobra.Command, args *general.ArgInfo) error {
	msg := "all " + CustomDataKind().Kind
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", CustomDataKind().Kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.DescribeCmd, err, msg)
	}

	body, err := httpGetCustomDataKind(args.Name)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kinds, err := general.UnmarshalMapInterface(body, !args.ContainName())
	if err != nil {
		return getErr(err)
	}
	// Output:
	// Name: customdatakindxxx
	// ...
	general.PrintMapInterface(kinds, []string{"name"}, []string{})
	return nil
}

func httpGetCustomDataKind(name string) ([]byte, error) {
	url := func(name string) string {
		if len(name) == 0 {
			return makePath(general.CustomDataKindURL)
		}
		return makePath(general.CustomDataKindItemURL, name)
	}(name)

	return handleReq(http.MethodGet, url, nil)
}

// GetCustomDataKind returns the custom data kind.
func GetCustomDataKind(cmd *cobra.Command, args *general.ArgInfo) error {
	msg := "all " + CustomDataKind().Kind
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", CustomDataKind().Kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetCustomDataKind(args.Name)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kinds, err := unmarshalCustomDataKind(body, !args.ContainName())
	if err != nil {
		return getErr(err)
	}

	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i].Name < kinds[j].Name
	})
	printCustomDataKinds(kinds)
	return nil
}

// EditCustomDataKind edit the custom data kind.
func EditCustomDataKind(cmd *cobra.Command, args *general.ArgInfo) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.EditCmd, err, CustomDataKind().Kind, args.Name)
	}

	var oldYaml string
	var err error
	if oldYaml, err = getCustomDataKindYaml(args.Name); err != nil {
		return getErr(err)
	}
	filePath := getResourceTempFilePath(CustomDataKind().Kind, args.Name)
	newYaml, err := editResource(oldYaml, filePath)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	if newYaml == "" {
		return nil
	}

	_, newSpec, err := general.CompareYamlNameKind(oldYaml, newYaml)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	err = ApplyCustomDataKind(cmd, newSpec)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	return nil
}

func getCustomDataKindYaml(kindName string) (string, error) {
	body, err := httpGetCustomDataKind(kindName)
	if err != nil {
		return "", err
	}

	yamlBody, err := codectool.JSONToYAML(body)
	if err != nil {
		return "", err
	}

	// reorder yaml, put name, kind in front of other fields.
	lines := strings.Split(string(yamlBody), "\n")
	var name string
	var sb strings.Builder
	sb.Grow(len(yamlBody))
	for _, l := range lines {
		if strings.HasPrefix(l, "name:") {
			name = l
		} else {
			sb.WriteString(l)
			sb.WriteString("\n")
		}
	}
	return fmt.Sprintf("%s\n%s\n\n%s", name, "kind: CustomDataKind", sb.String()), nil
}

func unmarshalCustomDataKind(body []byte, listBody bool) ([]*customdata.KindWithLen, error) {
	if listBody {
		metas := []*customdata.KindWithLen{}
		err := codectool.Unmarshal(body, &metas)
		return metas, err
	}
	meta := &customdata.KindWithLen{}
	err := codectool.Unmarshal(body, meta)
	return []*customdata.KindWithLen{meta}, err
}

func printCustomDataKinds(kinds []*customdata.KindWithLen) {
	// Output:
	// NAME   ID-FIELD    JSON-SCHEMA    DATA-NUM
	// xxx    - or name   yes/no         10
	table := [][]string{}
	table = append(table, []string{"NAME", "ID-FIELD", "JSON-SCHEMA", "DATA-NUM"})

	getRow := func(kind *customdata.KindWithLen) []string {
		jsonSchema := "no"
		if kind.JSONSchema != nil {
			jsonSchema = "yes"
		}
		idField := "-"
		if kind.IDField != "" {
			idField = kind.IDField
		}
		return []string{kind.Name, idField, jsonSchema, strconv.Itoa(kind.Len)}
	}

	for _, kind := range kinds {
		table = append(table, getRow(kind))
	}
	general.PrintTable(table)
}

// CreateCustomDataKind creates the custom data kind.
func CreateCustomDataKind(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makePath(general.CustomDataKindURL), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.CreateCmd, err, s.Kind, s.Name)
	}
	fmt.Println(general.SuccessMsg(general.CreateCmd, s.Kind, s.Name))
	return nil
}

// DeleteCustomDataKind deletes the custom data kind.
func DeleteCustomDataKind(cmd *cobra.Command, names []string, all bool) error {
	if all {
		_, err := handleReq(http.MethodDelete, makePath(general.CustomDataKindURL), nil)
		if err != nil {
			return general.ErrorMsg(general.DeleteCmd, err, "all", CustomDataKind().Kind)
		}
		fmt.Println(general.SuccessMsg(general.DeleteCmd, "all", CustomDataKind().Kind))
		return nil
	}

	for _, name := range names {
		_, err := handleReq(http.MethodDelete, makePath(general.CustomDataKindItemURL, name), nil)
		if err != nil {
			return general.ErrorMsg(general.DeleteCmd, err, CustomDataKind().Kind, name)
		}
		fmt.Println(general.SuccessMsg(general.DeleteCmd, CustomDataKind().Kind, name))
	}
	return nil
}

// ApplyCustomDataKind applies the custom data kind.
func ApplyCustomDataKind(cmd *cobra.Command, s *general.Spec) error {
	checkKindExist := func(cmd *cobra.Command, name string) bool {
		_, err := httpGetCustomDataKind(name)
		return err == nil
	}

	createOrUpdate := func(cmd *cobra.Command, yamlDoc []byte, exist bool) error {
		if exist {
			_, err := handleReq(http.MethodPut, makePath(general.CustomDataKindURL), yamlDoc)
			return err
		}
		_, err := handleReq(http.MethodPost, makePath(general.CustomDataKindURL), yamlDoc)
		return err
	}

	exist := checkKindExist(cmd, s.Name)
	action := general.CreateCmd
	if exist {
		action = "update"
	}

	err := createOrUpdate(cmd, []byte(s.Doc()), exist)
	if err != nil {
		return general.ErrorMsg(action, err, s.Kind, s.Name)
	}

	fmt.Println(general.SuccessMsg(action, s.Kind, s.Name))
	return nil
}

func getCertainCustomDataKind(kindName string) (*customdata.KindWithLen, error) {
	body, err := httpGetCustomDataKind(kindName)
	if err != nil {
		return nil, err
	}
	kinds, err := unmarshalCustomDataKind(body, false)
	if err != nil {
		return nil, err
	}
	return kinds[0], err
}
