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
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

// MemberCmd defines member command.
func MemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "(Deprecated) View Easegress members",
	}

	cmd.AddCommand(listMemberCmd())
	cmd.AddCommand(purgeMemberCmd())
	return cmd
}

func listMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Easegress members",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makePath(membersURL), nil, cmd)
		},
	}

	return cmd
}

func purgeMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "purge <member name>",
		Short:   "Purge a Easegress member",
		Long:    "Purge a Easegress member. This command should be run after the easegress node uninstalled",
		Example: "egctl member purge <member name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one member name to be deleted")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makePath(memberURL, args[0]), nil, cmd)
		},
	}

	return cmd
}
