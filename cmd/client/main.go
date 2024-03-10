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

// Package main is the entry point of Easegress client.
package main

import (
	"github.com/spf13/cobra"

	"github.com/megaease/easegress/v2/cmd/client/command"
	"github.com/megaease/easegress/v2/cmd/client/commandv2"
	"github.com/megaease/easegress/v2/cmd/client/general"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var deprecatedGroup = &cobra.Group{
	ID:    "deprecated",
	Title: `Deprecated Commands`,
}

var basicGroup = &cobra.Group{
	ID:    "basic",
	Title: `Basic Commands`,
}

var advancedGroup = &cobra.Group{
	ID:    "advanced",
	Title: `Advanced Commands`,
}

var otherGroup = &cobra.Group{
	ID:    "other",
	Title: `Other Commands`,
}

func main() {
	rootCmd := &cobra.Command{
		Use:        "egctl",
		Short:      "A command line admin tool for Easegress.",
		SuggestFor: []string{"egctl"},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			switch general.CmdGlobalFlags.OutputFormat {
			case "yaml", "json", "default":
			default:
				general.ExitWithErrorf("unsupported output format: %s, only support (yaml, json and default)",
					general.CmdGlobalFlags.OutputFormat)
			}
		},
	}

	addCommandWithGroup := func(group *cobra.Group, cmd ...*cobra.Command) {
		for _, c := range cmd {
			c.GroupID = group.ID
		}
		rootCmd.AddCommand(cmd...)
	}

	addCommandWithGroup(
		basicGroup,
		commandv2.CreateCmd(),
		commandv2.DeleteCmd(),
		commandv2.GetCmd(),
		commandv2.DescribeCmd(),
		commandv2.ApplyCmd(),
		commandv2.EditCmd(),
	)

	addCommandWithGroup(
		advancedGroup,
		commandv2.ConvertCmd(),
	)

	addCommandWithGroup(
		otherGroup,
		commandv2.APIsCmd(),
		commandv2.CompletionCmd(),
		commandv2.HealthCmd(),
		commandv2.ProfileCmd(),
		commandv2.APIResourcesCmd(),
		commandv2.WasmCmd(),
		commandv2.ConfigCmd(),
		commandv2.LogsCmd(),
		commandv2.MetricsCmd(),
	)

	addCommandWithGroup(
		deprecatedGroup,
		command.APICmd(),
		command.ObjectCmd(),
		command.MemberCmd(),
		command.CustomDataKindCmd(),
		command.CustomDataCmd(),
	)

	rootCmd.AddGroup(basicGroup, advancedGroup, otherGroup, deprecatedGroup)

	for _, c := range rootCmd.Commands() {
		general.GenerateExampleFromChild(c)
	}

	rootCmd.PersistentFlags().StringVar(&general.CmdGlobalFlags.Server,
		"server", "", "The address of the Easegress endpoint")
	rootCmd.PersistentFlags().BoolVar(&general.CmdGlobalFlags.ForceTLS,
		"force-tls", false, "Whether to forcibly use HTTPS, if not, client will auto upgrade to HTTPS on-demand")
	rootCmd.PersistentFlags().BoolVar(&general.CmdGlobalFlags.InsecureSkipVerify,
		"insecure-skip-verify", false, "Whether to verify the server's certificate chain and host name")
	rootCmd.PersistentFlags().StringVarP(&general.CmdGlobalFlags.OutputFormat,
		"output", "o", general.DefaultFormat, "Output format(default, json, yaml)")

	// since we have our own error handling, silence Cobra's error handling.
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil {
		general.ExitWithError(err)
	}
}
