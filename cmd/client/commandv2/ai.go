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
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/spf13/cobra"
)

// AICmd returns AI command.
func AICmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Get AI status", Command: "egctl ai status"},
		{Desc: "Get AI statistics", Command: "egctl ai stat"},
		{Desc: "Enable AI (Create the AIGatewayController)", Command: "egctl enable"},
		{Desc: "Disable AI (Delete the AIGatewayController)", Command: "egctl disable"},
	}

	cmd := &cobra.Command{
		Use:     "ai",
		Short:   "Commands to manage AI Gateway",
		Args:    cobra.NoArgs,
		Example: createMultiExample(examples),
		Run:     aiCmdRun,
	}

	cmd.AddCommand(
		enableCmd(),
		disableCmd(),
		statusCmd(),
		statCmd(),
	)

	return cmd
}

func enableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable AI Gateway (create AIGatewayController)",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			defer func() {
				if err != nil {
					general.ExitWithError(err)
				}
			}()

			s, err := general.GetSpecFromYaml(`kind: AIGatewayController
name: AIGatewayController
providers:
`)
			if err != nil {
				return
			}

			err = resources.CreateObject(cmd, s)
		},
	}
}

func disableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable AI Gateway (delete AIGatewayController)",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			defer func() {
				if err != nil {
					general.ExitWithError(err)
				}
			}()

			err = resources.DeleteObject(cmd, "AIGatewayController", []string{"AIGatewayController"}, false)
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Get AI Gateway status",
		Example: createExample("Get AI Gateway status.", "egctl ai status"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO
		},
	}
}

func statCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stat",
		Short:   "Get AI Gateway statistics",
		Example: createExample("Get AI Gateway statistics.", "egctl ai stat"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO
		},
	}
}

func aiCmdRun(cmd *cobra.Command, args []string) {
	cmd.Help()
}
