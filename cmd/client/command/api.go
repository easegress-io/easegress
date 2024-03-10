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

// Package command provides the commands.
package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

// APICmd defines API command.
func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "(Deprecated) View Easegress APIs",
	}

	cmd.AddCommand(listAPICmd())
	return cmd
}

func listAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Easegress APIs",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makePath(apiURL), nil, cmd)
		},
	}

	return cmd
}
