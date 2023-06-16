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
	"net/http"
	"strconv"
	"strings"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/cluster/customdata"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

const CustomDataKindName = "customdatakind"

func CustomDataKindAlias() []string {
	return []string{"customdatakinds", "cdk"}
}

func customDataKindCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return customDataKindGetCmd()
	default:
		return nil
	}
}

func customDataKindGetCmd() []*cobra.Command {
	return []*cobra.Command{getCustomDataKinds()}
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

const CustomDataName = "customdata"

func CustomDataAlias() []string {
	return []string{"customdatas", "cd"}
}

func customDataCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return customDataGetCmd()
	default:
		return nil
	}
}

func customDataGetCmd() []*cobra.Command {
	return []*cobra.Command{getCustomData()}
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

func getCustomData() *cobra.Command {
	getIDField := func(cmd *cobra.Command, args []string) (*customdata.KindWithLen, error) {
		body, err := httpGetCustomDataKind(cmd, args[0:1])
		if err != nil {
			return nil, err
		}
		kinds, err := unmarshalCustomDataKind(body, false)
		if err != nil {
			return nil, err
		}
		return kinds[0], err
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
		Run: func(cmd *cobra.Command, args []string) {
			body, err := httpGetCustomData(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kind, err := getIDField(cmd, args)
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
