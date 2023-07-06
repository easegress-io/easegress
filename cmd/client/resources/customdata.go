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

// Package resources provides the resources utilities for the client.
package resources

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/cluster/customdata"
	"github.com/megaease/easegress/pkg/util/codectool"
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

// CustomData is CustomData resource.
func CustomData() *api.APIResource {
	return &api.APIResource{
		Kind:    "CustomData",
		Name:    "customdata",
		Aliases: []string{"customdatas", "cd"},
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

	body, err := httpGetCustomDataKind(cmd, args.Name)
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

func httpGetCustomDataKind(cmd *cobra.Command, name string) ([]byte, error) {
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

	body, err := httpGetCustomDataKind(cmd, args.Name)
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
		_, err := httpGetCustomDataKind(cmd, name)
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

func httpGetCustomData(cmd *cobra.Command, args *general.ArgInfo) ([]byte, error) {
	url := func(args *general.ArgInfo) string {
		if !args.ContainOther() {
			return makePath(general.CustomDataURL, args.Name)
		}
		return makePath(general.CustomDataItemURL, args.Name, args.Other)
	}(args)

	return handleReq(http.MethodGet, url, nil)
}

func getCertainCustomDataKind(cmd *cobra.Command, kindName string) (*customdata.KindWithLen, error) {
	body, err := httpGetCustomDataKind(cmd, kindName)
	if err != nil {
		return nil, err
	}
	kinds, err := unmarshalCustomDataKind(body, false)
	if err != nil {
		return nil, err
	}
	return kinds[0], err
}

// GetCustomData gets the custom data.
func GetCustomData(cmd *cobra.Command, args *general.ArgInfo) error {
	msg := fmt.Sprintf("%s for kind %s", CustomData().Kind, args.Name)
	if args.ContainOther() {
		msg = fmt.Sprintf("%s with id %s for kind %s", CustomData().Kind, args.Other, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetCustomData(cmd, args)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kind, err := getCertainCustomDataKind(cmd, args.Name)
	if err != nil {
		return getErr(err)
	}

	data, err := unmarshalCustomData(body, !args.ContainOther())
	if err != nil {
		return getErr(err)
	}
	printCustomData(data, kind)
	return nil
}

// DescribeCustomData describes the custom data.
func DescribeCustomData(cmd *cobra.Command, args *general.ArgInfo) error {
	msg := fmt.Sprintf("%s for kind %s", CustomData().Kind, args.Name)
	if args.ContainOther() {
		msg = fmt.Sprintf("%s with id %s for kind %s", CustomData().Kind, args.Other, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.DescribeCmd, err, msg)
	}

	body, err := httpGetCustomData(cmd, args)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kind, err := getCertainCustomDataKind(cmd, args.Name)
	if err != nil {
		return getErr(err)
	}

	data, err := general.UnmarshalMapInterface(body, !args.ContainOther())
	if err != nil {
		return getErr(err)
	}
	general.PrintMapInterface(data, []string{kind.GetIDField()}, []string{})
	return nil
}

func unmarshalCustomData(body []byte, listBody bool) ([]*customdata.Data, error) {
	if listBody {
		metas := []*customdata.Data{}
		err := codectool.Unmarshal(body, &metas)
		return metas, err
	}
	meta := &customdata.Data{}
	err := codectool.Unmarshal(body, meta)
	return []*customdata.Data{meta}, err
}

func printCustomData(data []*customdata.Data, kind *customdata.KindWithLen) {
	table := [][]string{}
	table = append(table, []string{strings.ToUpper(kind.GetIDField())})

	for _, d := range data {
		table = append(table, []string{kind.DataID(d)})
	}
	general.PrintTable(table)
}

// CreateCustomData creates the custom data.
func CreateCustomData(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makePath(general.CustomDataItemURL, s.Name, "items"), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.CreateCmd, err, s.Kind, "for kind "+s.Name)
	}
	fmt.Println(general.SuccessMsg(general.CreateCmd, s.Kind, "for kind "+s.Name))
	return err
}

// DeleteCustomData deletes the custom data.
func DeleteCustomData(cmd *cobra.Command, cdk string, names []string, all bool) error {
	if all {
		_, err := handleReq(http.MethodDelete, makePath(general.CustomDataItemURL, cdk, "items"), nil)
		if err != nil {
			return general.ErrorMsg(general.DeleteCmd, err, CustomData().Kind, "for kind "+cdk)
		}
		fmt.Println(general.SuccessMsg(general.DeleteCmd, CustomData().Kind, "for kind "+cdk))
		return nil
	}

	for _, name := range names {
		_, err := handleReq(http.MethodDelete, makePath(general.CustomDataItemURL, cdk, name), nil)
		if err != nil {
			return general.ErrorMsg(general.DeleteCmd, err, CustomData().Kind, fmt.Sprintf("%s for kind %s", name, cdk))
		}
		fmt.Println(general.SuccessMsg(general.DeleteCmd, CustomData().Kind, fmt.Sprintf("%s for kind %s", name, cdk)))
	}
	return nil
}

// ApplyCustomData applies the custom data.
func ApplyCustomData(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makePath(general.CustomDataItemURL, s.Name, "items"), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.ApplyCmd, err, s.Kind, "for kind "+s.Name)
	}
	fmt.Println(general.SuccessMsg(general.ApplyCmd, s.Kind, "for kind "+s.Name))
	return nil
}
