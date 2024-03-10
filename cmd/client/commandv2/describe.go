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

var describeFlags resources.ObjectNamespaceFlags

// DescribeCmd returns describe command.
func DescribeCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Describe a resource with name", Command: "egctl describe <resource> <name>"},
		{Desc: "Describe all instances in that resource", Command: "egctl describe <resource>"},
		{Desc: "Describe a httpserver", Command: "egctl describe httpserver <name>"},
		{Desc: "Describe all pipelines", Command: "egctl describe pipeline"},
		{Desc: "Describe pipelines with verbose information", Command: "egctl describe pipeline -v"},
		{Desc: "Describe all members", Command: "egctl describe member"},
		{Desc: "Describe a customdata kind", Command: "egctl describe customdatakind <name>"},
		{Desc: "Describe a customdata of given kind", Command: "egctl describe customdata <kind> <name>"},
		{Desc: "Describe pipeline resources from all namespaces, including httpservers and pipelines created by IngressController, MeshController and GatewayController", Command: "egctl describe pipeline --all-namespaces"},
		{Desc: "Describe httpserver resources from a certain namespace", Command: "egctl describe httpserver --namespace <namespace>"},
		{Desc: "Check all possible api resources", Command: "egctl api-resources"},
	}
	cmd := &cobra.Command{
		Use:     "describe",
		Short:   "Show details of a specific resource or group of resources",
		Args:    describeCmdArgs,
		Example: createMultiExample(examples),
		Run:     describeCmdRun,
	}
	cmd.Flags().BoolVarP(&general.CmdGlobalFlags.Verbose, "verbose", "v", false, "Print verbose information")
	cmd.Flags().StringVar(&describeFlags.Namespace, "namespace", "",
		"namespace is used to describe httpservers and pipelines created by IngressController, MeshController or GatewayController"+
			"(these objects create httpservers and pipelines in an independent namespace)")
	cmd.Flags().BoolVar(&describeFlags.AllNamespace, "all-namespaces", false,
		"describe all resources in all namespaces (including the ones created by IngressController, MeshController and GatewayController that are in an independent namespace)")
	return cmd
}

func describeCmdRun(cmd *cobra.Command, args []string) {
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
		err = resources.DescribeCustomData(cmd, a)
	case resources.CustomDataKind().Kind:
		err = resources.DescribeCustomDataKind(cmd, a)
	case resources.Member().Kind:
		err = resources.DescribeMember(cmd, a)
	default:
		err = resources.DescribeObject(cmd, a, kind, &describeFlags)
	}
}

// one or two args, except customdata which allows three args
// egctl describe <resource>
// egctl describe <resource> <name>
// special:
// egctl describe customdata <kind> <name>
func describeCmdArgs(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
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
