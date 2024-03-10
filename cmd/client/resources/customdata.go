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
	"os"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/cluster/customdata"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// CustomData is CustomData resource.
func CustomData() *api.APIResource {
	return &api.APIResource{
		Kind:    "CustomData",
		Name:    "customdata",
		Aliases: []string{"customdatas", "cd"},
	}
}

func httpGetCustomData(args *general.ArgInfo) ([]byte, error) {
	url := func(args *general.ArgInfo) string {
		if !args.ContainOther() {
			return makePath(general.CustomDataURL, args.Name)
		}
		return makePath(general.CustomDataItemURL, args.Name, args.Other)
	}(args)

	return handleReq(http.MethodGet, url, nil)
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

	body, err := httpGetCustomData(args)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kind, err := getCertainCustomDataKind(args.Name)
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

	body, err := httpGetCustomData(args)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	kind, err := getCertainCustomDataKind(args.Name)
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

func EditCustomData(cmd *cobra.Command, args *general.ArgInfo) error {
	if args.ContainOther() {
		return editCustomDataItem(cmd, args)
	}
	return editCustomDataBatch(cmd, args)
}

func editCustomDataBatch(cmd *cobra.Command, args *general.ArgInfo) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.EditCmd, err, fmt.Sprintf("%s %s", CustomData().Kind, args.Name))
	}
	var oldYaml string
	var err error
	if oldYaml, err = getCustomDataBatchYaml(args); err != nil {
		return getErr(err)
	}
	filePath := getResourceTempFilePath(CustomData().Kind, args.Name)
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
	err = ApplyCustomData(cmd, newSpec)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	os.Remove(filePath)
	return nil
}

func getCustomDataBatchYaml(args *general.ArgInfo) (string, error) {
	body, err := httpGetCustomData(args)
	if err != nil {
		return "", err
	}

	batch := api.ChangeRequest{}
	err = codectool.Unmarshal(body, &batch.List)
	if err != nil {
		return "", err
	}
	batch.Rebuild = true
	yamlStr, err := codectool.MarshalYAML(batch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("name: %s\nkind: CustomData\n\n%s", args.Name, yamlStr), nil
}

func editCustomDataItem(cmd *cobra.Command, args *general.ArgInfo) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.EditCmd, err, fmt.Sprintf("%s %s %s", CustomData().Kind, args.Name, args.Other))
	}
	var oldYaml string
	var err error
	if oldYaml, err = getCustomDataItemYaml(args); err != nil {
		return getErr(err)
	}
	filePath := getResourceTempFilePath(CustomData().Kind, args.Name)
	newYaml, err := editResource(oldYaml, filePath)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	if newYaml == "" {
		return nil
	}

	if err = compareCustomDataID(args.Name, oldYaml, newYaml); err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	_, err = handleReq(http.MethodPut, makePath(general.CustomDataURL, args.Name), []byte(newYaml))
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	os.Remove(filePath)
	fmt.Println(general.SuccessMsg(general.EditCmd, CustomData().Kind, args.Name, args.Other))
	return nil
}

func compareCustomDataID(kindName, oldYaml, newYaml string) error {
	kind, err := getCertainCustomDataKind(kindName)
	if err != nil {
		return err
	}
	oldData := &customdata.Data{}
	newData := &customdata.Data{}
	err = codectool.Unmarshal([]byte(oldYaml), oldData)
	if err != nil {
		return err
	}
	err = codectool.Unmarshal([]byte(newYaml), newData)
	if err != nil {
		return err
	}
	if kind.DataID(oldData) != kind.DataID(newData) {
		fmt.Println(oldData, kind.DataID(oldData), newData, kind.DataID(newData))
		return fmt.Errorf("edit cannot change the %s of custom data", kind.GetIDField())
	}
	return nil
}

func getCustomDataItemYaml(args *general.ArgInfo) (string, error) {
	body, err := httpGetCustomData(args)
	if err != nil {
		return "", err
	}
	yamlBody, err := codectool.JSONToYAML(body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# edit CustomData for kind %s\n\n%s", args.Name, string(yamlBody)), nil
}
