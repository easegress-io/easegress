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
	"github.com/megaease/easegress/pkg/cluster/customdata"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// CustomDataKindName is the name of the custom data kind.
const CustomDataKindName = "customdatakind"

// CustomDataKindAlias is the alias of the custom data kind.
func CustomDataKindAlias() []string {
	return []string{"customdatakinds", "cdk"}
}

func customDataKindCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return customDataKindGetCmd()
	case general.DescribeCmd:
		return customDataKindDescribeCmd()
	case general.CreateCmd:
		return customDataKindCreateCmd()
	case general.DeleteCmd:
		return customDataKindDeleteCmd()
	case general.ApplyCmd:
		return customDataKindApplyCmd()
	default:
		return nil
	}
}

func customDataKindGetCmd() []*cobra.Command {
	return []*cobra.Command{getCustomDataKinds()}
}

func customDataKindDescribeCmd() []*cobra.Command {
	return []*cobra.Command{describeCustomDataKinds()}
}

func customDataKindDeleteCmd() []*cobra.Command {
	return []*cobra.Command{deleteCustomDataKind()}
}

func customDataKindApplyCmd() []*cobra.Command {
	return []*cobra.Command{applyCustomDataKind()}
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
			body, err := httpGetCustomDataKind(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kinds, err := general.UnmarshalMapInterface(body, len(args) == 0)
			if err != nil {
				general.ExitWithErrorf("Display custom data kinds failed: %v", err)
			}
			// Output:
			// Name: customdatakindxxx
			// ...
			general.PrintMapInterface(kinds, []string{"name"})
		},
	}
	return cmd
}

func httpGetCustomDataKind(cmd *cobra.Command, args []string) ([]byte, error) {
	url := func(args []string) string {
		if len(args) == 0 {
			return makeURL(general.CustomDataKindURL)
		}
		return makeURL(general.CustomDataKindItemURL, args[0])
	}(args)

	return handleReq(http.MethodGet, url, nil, cmd)
}

func getCustomDataKinds() *cobra.Command {
	examples := []general.Example{
		{Desc: "Get all custom data kinds", Command: "egctl get customdatakind"},
		{Desc: "Get certain custom data kind", Command: "egctl get customdatakind <custom-data-kind>"},
	}

	cmd := &cobra.Command{
		Use:     CustomDataKindName,
		Short:   "Display one or many custom data kinds",
		Aliases: CustomDataKindAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("requires at most one arg for custom data kind name")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := httpGetCustomDataKind(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kinds, err := unmarshalCustomDataKind(body, len(args) == 0)
			if err != nil {
				general.ExitWithErrorf("Display custom data kinds failed: %v", err)
			}

			sort.Slice(kinds, func(i, j int) bool {
				return kinds[i].Name < kinds[j].Name
			})
			printCustomDataKinds(kinds)
		},
	}
	return cmd
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

func customDataKindCreateCmd() []*cobra.Command {
	return []*cobra.Command{createCustomDataKind()}
}

func createCustomDataKind() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     CustomDataKindName,
		Short:   "Create a custom data kind from a yaml file or stdin",
		Aliases: CustomDataKindAlias(),
		Example: createExample("Create a custom data kind from a yaml file", "egctl create -f <custom-data-kind>.yaml"),
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				_, err := handleReq(http.MethodPost, makeURL(general.CustomDataKindURL), yamlDoc, cmd)
				if err != nil {
					general.ExitWithError(err)
				} else {
					fmt.Printf("Custom data kind created\n")
				}
				return err
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file containing custom data kind spec")
	return cmd
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
			_, err := handleReq(http.MethodDelete, makeURL(general.CustomDataKindItemURL, args[0]), nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			} else {
				fmt.Printf("Custom data kind %s deleted\n", args[0])
			}
		},
	}
	return cmd
}

func applyCustomDataKind() *cobra.Command {
	var specFile string

	getKind := func(yamlDoc []byte) (*customdata.Kind, error) {
		kind := &customdata.Kind{}
		err := codectool.Unmarshal(yamlDoc, kind)
		return kind, err
	}

	checkKindExist := func(cmd *cobra.Command, name string) bool {
		_, err := httpGetCustomDataKind(cmd, []string{name})
		return err == nil
	}

	createOrUpdate := func(cmd *cobra.Command, yamlDoc []byte, exist bool) error {
		if exist {
			_, err := handleReq(http.MethodPut, makeURL(general.CustomDataKindURL), yamlDoc, cmd)
			return err
		}
		_, err := handleReq(http.MethodPost, makeURL(general.CustomDataKindURL), yamlDoc, cmd)
		return err
	}

	cmd := &cobra.Command{
		Use:     CustomDataKindName,
		Short:   "Apply a custom data kind from a yaml file or stdin",
		Aliases: CustomDataKindAlias(),
		Example: createExample("Apply a custom data kind from a yaml file", "egctl apply customdatakind -f <custom-data-kind>.yaml"),
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				kind, err := getKind(yamlDoc)
				if err != nil {
					general.ExitWithError(err)
				}
				exist := checkKindExist(cmd, kind.Name)
				err = createOrUpdate(cmd, yamlDoc, exist)
				if err != nil {
					general.ExitWithError(err)
					return err
				}

				if exist {
					fmt.Printf("Custom data kind %s updated\n", kind.Name)
				} else {
					fmt.Printf("Custom data kind %s created\n", kind.Name)
				}
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

// CustomDataName is the name of custom data command
const CustomDataName = "customdata"

// CustomDataAlias is the alias of custom data command
func CustomDataAlias() []string {
	return []string{"customdatas", "cd"}
}

func customDataCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return customDataGetCmd()
	case general.DescribeCmd:
		return customDataDescribeCmd()
	case general.CreateCmd:
		return customDataCreateCmd()
	case general.DeleteCmd:
		return customDataDeleteCmd()
	case general.ApplyCmd:
		return customDataApplyCmd()
	default:
		return nil
	}
}

func customDataGetCmd() []*cobra.Command {
	return []*cobra.Command{getCustomData()}
}

func customDataDescribeCmd() []*cobra.Command {
	return []*cobra.Command{describeCustomData()}
}

func customDataCreateCmd() []*cobra.Command {
	return []*cobra.Command{createCustomData()}
}

func customDataDeleteCmd() []*cobra.Command {
	return []*cobra.Command{deleteCustomData()}
}

func customDataApplyCmd() []*cobra.Command {
	return []*cobra.Command{applyCustomData()}
}

func httpGetCustomData(cmd *cobra.Command, args []string) ([]byte, error) {
	url := func(args []string) string {
		if len(args) == 1 {
			return makeURL(general.CustomDataURL, args[0])
		}
		return makeURL(general.CustomDataItemURL, args[0], args[1])
	}(args)

	return handleReq(http.MethodGet, url, nil, cmd)
}

func getCertainCustomDataKind(cmd *cobra.Command, kindName string) (*customdata.KindWithLen, error) {
	body, err := httpGetCustomDataKind(cmd, []string{kindName})
	if err != nil {
		return nil, err
	}
	kinds, err := unmarshalCustomDataKind(body, false)
	if err != nil {
		return nil, err
	}
	return kinds[0], err
}

func getCustomData() *cobra.Command {
	examples := []general.Example{
		{Desc: "Get all custom data of certain kind", Command: "egctl get customdata <custom-data-kind>"},
		{Desc: "Get a certain custom data of certain kind", Command: "egctl get customdata <custom-data-kind> <custom-data-id>"},
	}

	cmd := &cobra.Command{
		Use:     CustomDataName,
		Short:   "Display one or many custom data of given kind",
		Aliases: CustomDataAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args) > 2 {
				return errors.New("requires at most two arg for custom data kind name and id")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := httpGetCustomData(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kind, err := getCertainCustomDataKind(cmd, args[0])
			if err != nil {
				general.ExitWithError(err)
				return
			}

			data, err := unmarshalCustomData(body, len(args) == 1)
			if err != nil {
				general.ExitWithErrorf("Display custom data failed: %v", err)
			}
			printCustomData(data, kind)
		},
	}
	return cmd
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
			body, err := httpGetCustomData(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kind, err := getCertainCustomDataKind(cmd, args[0])
			if err != nil {
				general.ExitWithError(err)
				return
			}

			data, err := general.UnmarshalMapInterface(body, len(args) == 1)
			if err != nil {
				general.ExitWithErrorf("Display custom data failed: %v", err)
			}
			general.PrintMapInterface(data, []string{kind.GetIDField()})
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

func createCustomData() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     CustomDataName,
		Short:   "Create a custom data from a yaml file or stdin",
		Aliases: CustomDataAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Example: createExample("Create a custom data from a yaml file", "egctl create customdata <custom-data-kind> -f <custom-data>.yaml"),
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				_, err := handleReq(http.MethodPost, makeURL(general.CustomDataURL, args[0]), yamlDoc, cmd)
				if err != nil {
					general.ExitWithError(err)
				} else {
					fmt.Println("Custom data created.")
				}
				return err
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file containing custom data spec")
	return cmd
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
			_, err := handleReq(http.MethodDelete, makeURL(general.CustomDataItemURL, args[0], args[1]), nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			} else {
				fmt.Printf("Custom data %s from %s deleted.", args[1], args[0])
			}
		},
	}

	return cmd
}

func applyCustomData() *cobra.Command {
	examples := []general.Example{
		{Desc: "Apply a custom data from a yaml file", Command: "egctl apply customdata <custom-data-kind> -f <custom-data>.yaml"},
		{Desc: "Apply a batch update for custom data", Command: "egctl apply customdata <custom-data-kind> -f <custom-data>.yaml --batch-update"},
	}

	var specFile string
	var batchUpdate bool

	getKind := func(cmd *cobra.Command, name string) *customdata.Kind {
		body, err := httpGetCustomDataKind(cmd, []string{name})
		if err != nil {
			general.ExitWithErrorf("Get custom data kind failed: %v", err)
		}
		kind := &customdata.Kind{}
		err = codectool.Unmarshal(body, kind)
		if err != nil {
			general.ExitWithErrorf("Unmarshal custom data kind failed: %v", err)
		}
		return kind
	}

	getDataName := func(cmd *cobra.Command, kind *customdata.Kind, data []byte) string {
		d := &customdata.Data{}
		err := codectool.Unmarshal(data, d)
		if err != nil {
			general.ExitWithErrorf("Unmarshal custom data failed: %v", err)
		}
		return kind.DataID(d)
	}

	dataExist := func(cmd *cobra.Command, kindName, dataName string) bool {
		body, err := httpGetCustomData(cmd, []string{kindName, dataName})
		return err == nil && len(body) > 0
	}

	applySingle := func(cmd *cobra.Command, args []string) {
		kind := getKind(cmd, args[0])
		visitor := buildYAMLVisitor(specFile, cmd)

		visitor.Visit(func(yamlDoc []byte) error {
			dataName := getDataName(cmd, kind, yamlDoc)
			if !dataExist(cmd, args[0], dataName) {
				_, err := handleReq(http.MethodPost, makeURL(general.CustomDataURL, args[0]), yamlDoc, cmd)
				if err != nil {
					general.ExitWithError(err)
				}
				fmt.Println("Custom data created.")
				return err
			}

			_, err := handleReq(http.MethodPut, makeURL(general.CustomDataURL, args[0]), yamlDoc, cmd)
			if err != nil {
				general.ExitWithError(err)
			}

			fmt.Println("Custom data updated.")
			return err
		})
		visitor.Close()
	}

	applyBatch := func(cmd *cobra.Command, args []string) {
		visitor := buildYAMLVisitor(specFile, cmd)
		visitor.Visit(func(yamlDoc []byte) error {
			_, err := handleReq(http.MethodPost, makeURL(general.CustomDataItemURL, args[0], "items"), yamlDoc, cmd)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println("Custom data batch updated.")
			return err
		})
		visitor.Close()
	}

	cmd := &cobra.Command{
		Use:     CustomDataName,
		Short:   "Apply custom data from a yaml file or stdin",
		Aliases: CustomDataAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			if batchUpdate {
				applyBatch(cmd, args)
			} else {
				applySingle(cmd, args)
			}
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file containing custom data spec")
	cmd.Flags().BoolVar(&batchUpdate, "batch-update", false, "Batch update custom data")
	return cmd
}
