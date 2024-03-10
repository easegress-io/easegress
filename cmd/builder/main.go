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

// Package main is the entry point of egbuilder.
package main

import (
	"github.com/megaease/easegress/v2/cmd/builder/command"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var basicGroup = &cobra.Group{
	ID:    "basic",
	Title: `Basic Commands`,
}

func main() {
	rootCmd := &cobra.Command{
		Use:        "egbuilder",
		Short:      "A command-line tool designed for building Easegress using custom modules.",
		SuggestFor: []string{"egbuilder"},
	}

	addCommandWithGroup := func(group *cobra.Group, cmd ...*cobra.Command) {
		for _, c := range cmd {
			c.GroupID = group.ID
		}
		rootCmd.AddCommand(cmd...)
	}

	addCommandWithGroup(
		basicGroup,
		command.InitCmd(),
		command.AddCmd(),
		command.BuildCmd(),
		command.RunCmd(),
	)

	rootCmd.AddGroup(basicGroup)

	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil {
		utils.ExitWithError(err)
	}
}
