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

package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

// APICmd defines API command.
func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "View EaseGateway APIs",
	}

	cmd.AddCommand(listAPICmd())
	return cmd
}

func listAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List EaseGateway APIs",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(apiURL), nil, cmd)
		},
	}

	return cmd
}
