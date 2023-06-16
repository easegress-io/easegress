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

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

const ObjectName = "object"

func ObjectAlias() []string {
	return []string{"o", "obj", "objects"}
}

const ObjectKindName = "objectkind"

func ObjectKindAlias() []string {
	return []string{"objectkinds", "ok"}
}

func objectCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return objectGetCmd()
	default:
		return nil
	}
}

func objectGetCmd() []*cobra.Command {
	return []*cobra.Command{getObject(), getObjectKinds()}
}

func getObject() *cobra.Command {
	getURL := func(args []string) string {
		if len(args) == 0 {
			return makeURL(general.ObjectsURL)
		}
		return makeURL(general.ObjectURL, args[0])
	}

	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Display one or many objects",
		Aliases: ObjectAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("requires at most one arg for object name")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, getURL(args), nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			metas, err := unmarshalMetaSpec(body, len(args) == 0)
			if err != nil {
				general.ExitWithErrorf("Display objects failed: %v", err)
			}
			printMetaSpec(metas)
		},
	}
	return cmd
}

func getObjectKinds() *cobra.Command {
	cmd := &cobra.Command{
		Use:     ObjectKindName,
		Short:   "Display available object kinds",
		Aliases: ObjectKindAlias(),
		Args:    cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makeURL(general.ObjectKindsURL), nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kinds := []string{}
			err = codectool.Unmarshal(body, &kinds)
			if err != nil {
				general.ExitWithErrorf("Display object kinds failed: %v", err)
			}
			printObjectKinds(kinds)
		},
	}
	return cmd
}

func unmarshalMetaSpec(body []byte, listBody bool) ([]*supervisor.MetaSpec, error) {
	if listBody {
		metas := []*supervisor.MetaSpec{}
		err := codectool.Unmarshal(body, &metas)
		return metas, err
	}
	meta := &supervisor.MetaSpec{}
	err := codectool.Unmarshal(body, meta)
	return []*supervisor.MetaSpec{meta}, err
}

func printMetaSpec(metas []*supervisor.MetaSpec) {
	table := [][]string{}
	table = append(table, []string{"NAME", "KIND"})
	for _, meta := range metas {
		table = append(table, []string{meta.Name, meta.Kind})
	}
	general.PrintTable(table)
}

func printObjectKinds(kinds []string) {
	table := [][]string{}
	table = append(table, []string{"KIND"})
	for _, kind := range kinds {
		table = append(table, []string{kind})
	}
	general.PrintTable(table)
}
