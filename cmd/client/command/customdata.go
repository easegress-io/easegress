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

// CustomDataCmd defines custom data command.
func CustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-data",
		Short: "View and change custom data",
	}

	cmd.AddCommand(listCustomDataCmd())
	cmd.AddCommand(getCustomDataCmd())
	cmd.AddCommand(updateCustomDataCmd())

	return cmd
}

func getCustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an custom data",
		Example: "egctl custom-data get <kind> <id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires custom data kind and id to be retrieved")
			}
			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(customDataURL, args[0], args[1]), nil, cmd)
		},
	}

	return cmd
}

func listCustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all custom data of a kind",
		Example: "egctl custom-data list <kind>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(customDataKindURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func updateCustomDataCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Batch update custom data from a yaml file or stdin",
		Example: "egctl custom-data update <kind> -f <change request file>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPost, makeURL(customDataKindURL, args[0]), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}
