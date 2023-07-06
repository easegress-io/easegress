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

// Package commandv2 provides the new version of commands.
package commandv2

import (
	"errors"
	"fmt"
	"strings"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/cmd/client/resources"
	"github.com/spf13/cobra"
)

// GetCmd returns get command.
func GetCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Get a resource with name", Command: "egctl get <resource> <name>"},
		{Desc: "Get all resource", Command: "egctl get all"},
		{Desc: "Get a resource with yaml output", Command: "egctl get <resource> <name> -o yaml"},
		{Desc: "Get all instances in that resource", Command: "egctl get <resource>"},
		{Desc: "Get a httpserver", Command: "egctl get httpserver <name>"},
		{Desc: "Get all pipelines", Command: "egctl get pipeline"},
		{Desc: "Get all members", Command: "egctl get member"},
		{Desc: "Get a customdata kind", Command: "egctl get customdatakind <name>"},
		{Desc: "Get a customdata of given kind", Command: "egctl get customdata <kind> <name>"},
		{Desc: "Check all possible api resources", Command: "egctl api-resources"},
	}
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Display one or many resources",
		Args:    getCmdArgs,
		Example: createMultiExample(examples),
		Run:     getCmdRun,
	}
	return cmd
}

func getAllResources(cmd *cobra.Command) error {
	errs := []string{}
	appendErr := func(err error) {
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	err := resources.GetAllObject(cmd)
	if err != nil {
		appendErr(err)
	} else {
		fmt.Printf("\n")
	}

	funcs := []func(*cobra.Command, *general.ArgInfo) error{
		resources.GetMember, resources.GetCustomDataKind,
	}
	for _, f := range funcs {
		err = f(cmd, &general.ArgInfo{Resource: "all"})
		if err != nil {
			appendErr(err)
		} else {
			fmt.Printf("\n")
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

func getCmdRun(cmd *cobra.Command, args []string) {
	var err error
	defer func() {
		if err != nil {
			general.ExitWithError(err)
		}
	}()

	a := general.ParseArgs(args)
	if a.Resource == "all" {
		err = getAllResources(cmd)
		return
	}

	kind, err := resources.GetResourceKind(a.Resource)
	if err != nil {
		return
	}
	switch kind {
	case resources.CustomData().Kind:
		err = resources.GetCustomData(cmd, a)
	case resources.CustomDataKind().Kind:
		err = resources.GetCustomDataKind(cmd, a)
	case resources.Member().Kind:
		err = resources.GetMember(cmd, a)
	default:
		err = resources.GetObject(cmd, a, kind)
	}
}

// one or two args, except customdata which allows three args
// egctl get all
// egctl get <resource>
// egctl get <resource> <name>
// special:
// egctl get customdata <kind> <name>
func getCmdArgs(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		cmd.Help()
		return fmt.Errorf("no resource specified")
	}
	if len(args) == 1 {
		if general.InAPIResource(args[0], resources.CustomData()) {
			return fmt.Errorf("no custom data kind specified")
		}
		return nil
	}
	if args[0] == "all" && len(args) != 1 {
		return fmt.Errorf("no more args allowed for arg 'all'")
	}
	if len(args) == 2 {
		return nil
	}
	if len(args) == 3 && general.InAPIResource(args[0], resources.CustomData()) {
		return nil
	}
	return fmt.Errorf("invalid args")
}
