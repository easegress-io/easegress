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
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

func ingressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingress",
		Short: "query and manager ingress",
	}
	cmd.AddCommand(createIngressCmd())
	cmd.AddCommand(updateIngressCmd())
	cmd.AddCommand(deleteIngressCmd())
	cmd.AddCommand(getIngressCmd())
	cmd.AddCommand(listIngressCmd())
	return cmd
}

func createIngressCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a ingress from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshIngressURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the ingress.")

	return cmd
}

func updateIngressCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an ingress from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshIngressURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the ingress.")

	return cmd
}

func deleteIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an ingress",
		Example: "egctl mesh ingress delete <ingress_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one ingress name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshIngressURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an ingress",
		Example: "egctl mesh tenant get <ingress_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one ingress name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshIngressURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all ingress",
		Example: "egctl mesh ingress list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshIngressesURL), nil, cmd)
		},
	}

	return cmd
}
