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
	"errors"
	"fmt"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// ConfigCmd defines config command.
func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View egctl config",
	}

	cmd.AddCommand(configInfoCmd())
	cmd.AddCommand(configViewCmd())
	cmd.AddCommand(configGetContextsCmd())
	cmd.AddCommand(configUseContextCmd())
	return cmd
}

func configInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current-context",
		Short: "Show current used egctl config",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := general.GetCurrentConfig()
			if err != nil {
				general.ExitWithError(err)
			}
			if config == nil {
				fmt.Println("not find config file at ~/.egctlrc")
				return
			}
			data, err := codectool.MarshalJSON(config.Context)
			if err != nil {
				general.ExitWithError(err)
			}
			general.PrintBody(data)
		},
	}

	return cmd
}

func configViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Display merged egctl config settings",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := general.GetConfig()
			if err != nil {
				general.ExitWithError(err)
			}
			if config == nil {
				fmt.Println("not find config file at ~/.egctlrc")
				return
			}
			config = general.GetRedactedConfig(config)
			data, err := codectool.MarshalJSON(config)
			if err != nil {
				general.ExitWithError(err)
			}
			general.PrintBody(data)
		},
	}
	return cmd
}

func configGetContextsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-contexts",
		Short: "Display one or many contexts",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := general.GetConfig()
			if err != nil {
				general.ExitWithError(err)
			}
			if config == nil {
				fmt.Println("not find config file at ~/.egctlrc")
				return
			}

			context := config.Contexts
			if !general.CmdGlobalFlags.DefaultFormat() {
				data, err := codectool.MarshalJSON(context)
				if err != nil {
					general.ExitWithError(err)
				}
				general.PrintBody(data)
				return
			}

			table := [][]string{{"CURRENT", "NAME", "CLUSTER", "USER"}}
			for _, ctx := range context {
				current := ""
				if ctx.Name == config.CurrentContext {
					current = "*"
				}
				table = append(table, []string{current, ctx.Name, ctx.Context.Cluster, ctx.Context.AuthInfo})
			}
			general.PrintTable(table)
		},
	}
	return cmd
}

func configUseContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use-context",
		Short: "Sets the current-context in egctl config file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			config, err := general.GetConfig()
			if err != nil {
				general.ExitWithError(err)
			}
			if config == nil {
				general.ExitWithError(errors.New("not find config file at ~/.egctlrc"))
			}

			newContext := args[0]
			_, ok := general.Find(config.Contexts, func(ctx general.NamedContext) bool {
				return ctx.Name == newContext
			})
			if !ok {
				general.ExitWithError(fmt.Errorf("not find context %s", newContext))
			}

			config.CurrentContext = newContext
			err = general.WriteConfig(config)
			if err != nil {
				general.ExitWithError(fmt.Errorf("write config file failed: %v", err))
			}
			fmt.Printf("Switched to context %s\n", newContext)
		},
	}
	return cmd
}
