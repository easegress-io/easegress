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

// Package commandv2 provides the new version of commands.
package commandv2

import (
	"fmt"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/spf13/cobra"
)

var deleteAllFlag = false

// DeleteCmd returns delete command.
func DeleteCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Delete a resource with name", Command: "egctl delete <resource> <name>"},
		{Desc: "Delete multiply resource in same kind", Command: "egctl delete <resource> <name1> <name2> <name3>"},
		{Desc: "Delete all instances in that resource", Command: "egctl delete <resource> --all"},
		{Desc: "Delete a httpserver", Command: "egctl delete httpserver <name>"},
		{Desc: "Delete multiply pipeline", Command: "egctl delete pipeline <name1> <name2> <name3>"},
		{Desc: "Delete all globalfilter", Command: "egctl delete globalfilter --all"},
		{Desc: "Delete a customdata kind", Command: "egctl delete customdatakind <name>"},
		{Desc: "Delete a customdata of given kind", Command: "egctl delete customdata <kind> <name>"},
		{Desc: "Purge a Easegress member. This command should be run after the easegress node uninstalled", Command: "egctl delete member <name>"},
		{Desc: "Check all possible api resources", Command: "egctl api-resources"},
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource or group of resources",
		Args:    deleteCmdArgs,
		Example: createMultiExample(examples),
		Run:     deleteCmdRun,
	}
	cmd.Flags().BoolVar(&deleteAllFlag, "all", false, "delete all resources in given kind")
	return cmd
}

func deleteCmdRun(cmd *cobra.Command, args []string) {
	var err error
	defer func() {
		if err != nil {
			general.ExitWithError(err)
		}
	}()

	resource := args[0]
	kind, err := resources.GetResourceKind(resource)
	if err != nil {
		return
	}

	switch kind {
	case resources.CustomData().Kind:
		err = resources.DeleteCustomData(cmd, args[1], args[2:], deleteAllFlag)
	case resources.CustomDataKind().Kind:
		err = resources.DeleteCustomDataKind(cmd, args[1:], deleteAllFlag)
	case resources.Member().Kind:
		err = resources.DeleteMember(cmd, args[1:])
	default:
		err = resources.DeleteObject(cmd, kind, args[1:], deleteAllFlag)
	}
}

// egctl delete <resource> <name1> <name2> ...
// egctl delete <resource> --all
// special:
// egctl delete cd <kind> <name1> <name2> ...
func deleteCmdArgs(_ *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no resource specified")
	}
	resource := args[0]
	kind, err := resources.GetResourceKind(resource)
	if err != nil {
		return fmt.Errorf("invalid resource for %s, %s", resource, err.Error())
	}

	if deleteAllFlag {
		if (kind != resources.CustomData().Kind && len(args) != 1) || (kind == resources.CustomData().Kind && len(args) != 2) {
			fmt.Printf("%s %s", kind, args)
			return fmt.Errorf("invalid arguments for --all")
		}
		return nil
	}

	if kind == resources.CustomData().Kind {
		if len(args) < 3 {
			return fmt.Errorf("")
		}
		return nil
	}
	if len(args) < 2 {
		return fmt.Errorf("invalid arguments")
	}
	return nil
}
