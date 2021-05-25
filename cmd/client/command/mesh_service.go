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

func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "query and manager service",
	}

	cmd.AddCommand(createServiceCmd())
	cmd.AddCommand(updateServiceCmd())
	cmd.AddCommand(deleteServiceCmd())
	cmd.AddCommand(getServiceCmd())
	cmd.AddCommand(listServicesCmd())
	cmd.AddCommand(serviceCanaryCmd())
	cmd.AddCommand(serviceResilienceCmd())
	cmd.AddCommand(serviceLoadbalanceCmd())
	cmd.AddCommand(serviceOutputserverCmd())
	cmd.AddCommand(serviceTracingsCmd())
	cmd.AddCommand(serviceMetricsCmd())
	cmd.AddCommand(serviceInstanceCmd())
	return cmd
}

func createServiceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service.")

	return cmd
}

func updateServiceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service.")

	return cmd
}

func deleteServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service",
		Example: "egctl mesh service delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service",
		Example: "egctl mesh service get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listServicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all services",
		Example: "egctl mesh service list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServicesURL), nil, cmd)
		},
	}

	return cmd
}
