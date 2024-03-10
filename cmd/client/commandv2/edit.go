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

package commandv2

import (
	"errors"
	"fmt"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/spf13/cobra"
)

// EditCmd returns edit command.
func EditCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Edit a resource with name", Command: "egctl edit <resource> <name>"},
		{Desc: "Edit a resource with nano", Command: "env EGCTL_EDITOR=nano egctl edit <resource> <name>"},
		{Desc: "Edit a httpserver with name", Command: "egctl edit httpserver httpserver-demo"},
		{Desc: "Edit all custom data with kind name", Command: "egctl edit customdata kind1"},
		{Desc: "Edit custom data with kind name and id name", Command: "egctl edit customdata kind1 data1"},
	}
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "Edit a resource",
		Args:    editCmdArgs,
		Example: createMultiExample(examples),
		Run:     editCmdRun,
	}
	return cmd
}

func editCmdRun(cmd *cobra.Command, args []string) {
	var err error
	defer func() {
		if err != nil {
			general.ExitWithError(err)
		}
	}()

	a := general.ParseArgs(args)

	kind, err := resources.GetResourceKind(a.Resource)
	if err != nil {
		return
	}
	switch kind {
	case resources.CustomData().Kind:
		err = resources.EditCustomData(cmd, a)
	case resources.CustomDataKind().Kind:
		err = resources.EditCustomDataKind(cmd, a)
	case resources.Member().Kind:
		err = errors.New("cannot edit member")
	default:
		err = resources.EditObject(cmd, a, kind)
	}
}

// editCmdArgs checks if args are valid.
// egctl edit <resource> <name>
// special:
// egctl edit customdata <kind> <name>
func editCmdArgs(cmd *cobra.Command, args []string) (err error) {
	if len(args) <= 1 {
		return fmt.Errorf("no resource and name specified")
	}
	if len(args) == 2 {
		return nil
	}
	if len(args) == 3 && general.InAPIResource(args[0], resources.CustomData()) {
		return nil
	}
	return fmt.Errorf("invalid args")
}
