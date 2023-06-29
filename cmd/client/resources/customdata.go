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

package resources

import (
	"errors"
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

func CustomDataKind() *api.ApiResource {
	return &api.ApiResource{
		Kind:    "CustomDataKind",
		Name:    "customdatakind",
		Aliases: []string{"customdatakinds", "cdk"},
	}
}

func CustomData() *api.ApiResource {
	return &api.ApiResource{
		Kind:    "CustomData",
		Name:    "customdata",
		Aliases: []string{"customdatas", "cd"},
	}
}

// CustomDataKindName is the name of the custom data kind.
const CustomDataKindName = "customdatakind"

// CustomDataKindAlias is the alias of the custom data kind.
func CustomDataKindAlias() []string {
	return []string{"customdatakinds", "cdk"}
}

func customDataKindCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.DescribeCmd:
		return customDataKindDescribeCmd()
	case general.DeleteCmd:
		return customDataKindDeleteCmd()
	default:
		return nil
	}
}

func customDataKindDescribeCmd() []*cobra.Command {
	return []*cobra.Command{describeCustomDataKinds()}
}

func customDataKindDeleteCmd() []*cobra.Command {
	return []*cobra.Command{deleteCustomDataKind()}
}

func describeCustomDataKinds() *cobra.Command {
	examples := []general.Example{
		{Desc: "Describe all custom data kinds", Command: "egctl describe customdatakind"},
		{Desc: "Describe certain custom data kind", Command: "egctl describe customdatakind <custom-data-kind>"},
	}

	cmd := &cobra.Command{
		Use:     CustomDataKindName,
		Short:   "Describe one or many custom data kinds",
		Aliases: CustomDataKindAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("requires at most one arg for custom data kind name")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			// body, err := httpGetCustomDataKind(cmd, args)
			// if err != nil {
			// 	general.ExitWithError(err)
			// }

			// if !general.CmdGlobalFlags.DefaultFormat() {
			// 	general.PrintBody(body)
			// 	return
			// }

			// kinds, err := general.UnmarshalMapInterface(body, len(args) == 0)
			// if err != nil {
			// 	general.ExitWithErrorf("Display custom data kinds failed: %v", err)
			// }
			// // Output:
			// // Name: customdatakindxxx
			// // ...
			// general.PrintMapInterface(kinds, []string{"name"})
		},
	}
	return cmd
}

func httpGetCustomDataKind(cmd *cobra.Command, name string) ([]byte, error) {
	url := func(name string) string {
		if len(name) == 0 {
			return makeURL(general.CustomDataKindURL)
		}
		return makeURL(general.CustomDataKindItemURL, name)
	}(name)

	return handleReq(http.MethodGet, url, nil)
}

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

func CreateCustomDataKind(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makeURL(general.CustomDataKindURL), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.CreateCmd, err, s.Kind, s.Name)
	}
	fmt.Println(general.SuccessMsg(general.CreateCmd, s.Kind, s.Name))
	return nil
}

func deleteCustomDataKind() *cobra.Command {
	cmd := &cobra.Command{
		Use:     CustomDataKindName,
		Short:   "Delete a custom data kind",
		Aliases: CustomDataKindAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Example: createExample("Delete a custom data kind", "egctl delete customdatakind <custom-data-kind>"),
		Run: func(cmd *cobra.Command, args []string) {
			_, err := handleReq(http.MethodDelete, makeURL(general.CustomDataKindItemURL, args[0]), nil)
			if err != nil {
				general.ExitWithError(err)
			} else {
				fmt.Printf("Custom data kind %s deleted\n", args[0])
			}
		},
	}
	return cmd
}

func ApplyCustomDataKind(cmd *cobra.Command, s *general.Spec) error {
	checkKindExist := func(cmd *cobra.Command, name string) bool {
		_, err := httpGetCustomDataKind(cmd, name)
		return err == nil
	}

	createOrUpdate := func(cmd *cobra.Command, yamlDoc []byte, exist bool) error {
		if exist {
			_, err := handleReq(http.MethodPut, makeURL(general.CustomDataKindURL), yamlDoc)
			return err
		}
		_, err := handleReq(http.MethodPost, makeURL(general.CustomDataKindURL), yamlDoc)
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

// CustomDataName is the name of custom data command
const CustomDataName = "customdata"

// CustomDataAlias is the alias of custom data command
func CustomDataAlias() []string {
	return []string{"customdatas", "cd"}
}

func customDataCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.DescribeCmd:
		return customDataDescribeCmd()
	case general.DeleteCmd:
		return customDataDeleteCmd()
	default:
		return nil
	}
}

func customDataDescribeCmd() []*cobra.Command {
	return []*cobra.Command{describeCustomData()}
}

func customDataDeleteCmd() []*cobra.Command {
	return []*cobra.Command{deleteCustomData()}
}

func httpGetCustomData(cmd *cobra.Command, args *general.ArgInfo) ([]byte, error) {
	url := func(args *general.ArgInfo) string {
		if !args.ContainOther() {
			return makeURL(general.CustomDataURL, args.Name)
		}
		return makeURL(general.CustomDataItemURL, args.Name, args.Other)
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

func describeCustomData() *cobra.Command {
	examples := []general.Example{
		{Desc: "Describe all custom data of certain kind", Command: "egctl describe customdata <custom-data-kind>"},
		{Desc: "Describe a certain custom data of certain kind", Command: "egctl describe customdata <custom-data-kind> <custom-data-id>"},
	}

	cmd := &cobra.Command{
		Use:     CustomDataName,
		Short:   "Describe one or many custom data of given kind",
		Aliases: CustomDataAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args) > 2 {
				return errors.New("requires at most two arg for custom data kind name and id")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			// body, err := httpGetCustomData(cmd, args)
			// if err != nil {
			// 	general.ExitWithError(err)
			// }

			// if !general.CmdGlobalFlags.DefaultFormat() {
			// 	general.PrintBody(body)
			// 	return
			// }

			// kind, err := getCertainCustomDataKind(cmd, args[0])
			// if err != nil {
			// 	general.ExitWithError(err)
			// 	return
			// }

			// data, err := general.UnmarshalMapInterface(body, len(args) == 1)
			// if err != nil {
			// 	general.ExitWithErrorf("Display custom data failed: %v", err)
			// }
			// general.PrintMapInterface(data, []string{kind.GetIDField()})
		},
	}
	return cmd
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

func CreateCustomData(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makeURL(general.CustomDataItemURL, s.Name, "items"), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.CreateCmd, err, s.Kind, "for kind "+s.Name)
	}
	fmt.Println(general.SuccessMsg(general.CreateCmd, s.Kind, "for kind "+s.Name))
	return err
}

func deleteCustomData() *cobra.Command {
	cmd := &cobra.Command{
		Use:     CustomDataName,
		Short:   "Delete a custom data item",
		Aliases: CustomDataAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires custom data kind and id to be retrieved")
			}
			return nil
		},
		Example: createExample("Delete a custom data item", "egctl delete customdata <custom-data-kind> <custom-data-id>"),
		Run: func(cmd *cobra.Command, args []string) {
			_, err := handleReq(http.MethodDelete, makeURL(general.CustomDataItemURL, args[0], args[1]), nil)
			if err != nil {
				general.ExitWithError(err)
			} else {
				fmt.Printf("Custom data %s from %s deleted.", args[1], args[0])
			}
		},
	}

	return cmd
}

func ApplyCustomData(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makeURL(general.CustomDataItemURL, s.Name, "items"), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.ApplyCmd, err, s.Kind, "for kind "+s.Name)
	}
	fmt.Println(general.SuccessMsg(general.ApplyCmd, s.Kind, "for kind "+s.Name))
	return nil
}
