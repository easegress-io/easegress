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
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// CompletionCmd returns completion command to generate completion script for the specified shell (bash or zsh).
func CompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion [bash|zsh]",
		Short:                 "Generates completion script for the specified shell (bash or zsh)",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			}
		},
	}
	return cmd
}

// HealthCmd returns health command.
func HealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "health",
		Short:   "Probe Easegress health",
		Example: createExample("Probe Easegress health", "egctl health"),
		Run: func(cmd *cobra.Command, args []string) {
			_, err := handleReq(http.MethodGet, makePath(general.HealthURL), nil)
			if err != nil {
				general.ExitWithError(err)
			}
			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody([]byte(`{"message": "OK"}`))
				return
			}
			fmt.Println("OK")
		},
	}

	return cmd
}

// APIsCmd returns apis command.
func APIsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apis",
		Short:   "View Easegress APIs",
		Example: createExample("List all apis", "egctl apis"),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makePath(general.APIURL), nil)
			if err != nil {
				general.ExitWithError(err)
			}
			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			groups := []*api.Group{}
			err = codectool.Unmarshal(body, &groups)
			if err != nil {
				general.ExitWithErrorf("unmarshal API groups failed: %v", err)
			}

			table := [][]string{}
			for _, group := range groups {
				for _, e := range group.Entries {
					table = append(table, []string{e.Path, e.Method, general.APIURL, group.Group})
				}
			}
			sort.Slice(table, func(i, j int) bool {
				return table[i][0] < table[j][0]
			})
			table = append([][]string{{"PATH", "METHOD", "VERSION", "GROUP"}}, table...)
			general.PrintTable(table)
		},
	}

	return cmd
}

// APIResourcesCmd returns api-resources command.
func APIResourcesCmd() *cobra.Command {
	cd := resources.CustomData()
	cdk := resources.CustomDataKind()
	member := resources.Member()

	cmd := &cobra.Command{
		Use:   "api-resources",
		Short: "View all API resources",
		Run: func(cmd *cobra.Command, args []string) {
			resources, err := resources.ObjectAPIResources()
			if err != nil {
				general.ExitWithError(err)
			}

			action := "create,apply,delete,get,describe"
			tables := [][]string{}
			for _, r := range resources {
				sort.Slice(r.Aliases, func(i, j int) bool {
					return len(r.Aliases[i]) < len(r.Aliases[j])
				})
				tables = append(tables, []string{r.Name, strings.Join(r.Aliases, ","), r.Category, r.Kind, action})
			}
			sort.Slice(tables, func(i, j int) bool {
				return tables[i][0] < tables[j][0]
			})

			tables = append([][]string{{"NAME", "ALIASES", "CATEGORY", "KIND", "ACTION"}}, tables...)
			tables = append(tables, []string{cdk.Name, strings.Join(cdk.Aliases, ","), cdk.Kind, action})
			tables = append(tables, []string{cd.Name, strings.Join(cd.Aliases, ","), cd.Kind, action})
			tables = append(tables, []string{member.Name, strings.Join(member.Aliases, ","), member.Kind, "delete,get,describe"})

			general.PrintTable(tables)
		},
	}
	return cmd
}

// MetricsCmd returns logs command.
func MetricsCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Print metrics of easegress and built-in etcd.", Command: "egctl metrics"},
	}

	cmd := &cobra.Command{
		Use:     "metrics",
		Short:   "Print metrics of easegress and built-in etcd.",
		Args:    cobra.NoArgs,
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := general.HandleRequest(http.MethodGet, general.MetricsURL, nil)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println(string(body))
		},
	}
	return cmd
}
