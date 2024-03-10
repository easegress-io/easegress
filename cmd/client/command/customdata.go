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

// CustomDataKindCmd defines custom data kind command.
func CustomDataKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-data-kind",
		Short: "(Deprecated) View and change custom data kind",
	}

	cmd.AddCommand(listCustomDataKindCmd())
	cmd.AddCommand(getCustomDataKindCmd())
	cmd.AddCommand(createCustomDataKindCmd())
	cmd.AddCommand(updateCustomDataKindCmd())
	cmd.AddCommand(deleteCustomDataKindCmd())

	return cmd
}

func listCustomDataKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all custom data kinds",
		Example: "egctl custom-data-kind list",

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makePath(customDataKindURL), nil, cmd)
		},
	}

	return cmd
}

func getCustomDataKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a custom data kind",
		Example: "egctl custom-data-kind get <kind>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makePath(customDataKindItemURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func createCustomDataKindCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a custom data kind from a yaml file or stdin",
		Example: "egctl custom-data-kind create -f <kind file>",
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPost, makePath(customDataKindURL), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

func updateCustomDataKindCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update a custom data from a yaml file or stdin",
		Example: "egctl custom-data-kind update -f <kind file>",
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPut, makePath(customDataKindURL), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

func deleteCustomDataKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a custom data kind",
		Example: "egctl custom-data-kind delete <kind>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makePath(customDataKindItemURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

// CustomDataCmd defines custom data command.
func CustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-data",
		Short: "(Deprecated) View and change custom data",
	}

	cmd.AddCommand(listCustomDataCmd())
	cmd.AddCommand(getCustomDataCmd())
	cmd.AddCommand(createCustomDataCmd())
	cmd.AddCommand(updateCustomDataCmd())
	cmd.AddCommand(batchUpdateCustomDataCmd())
	cmd.AddCommand(deleteCustomDataCmd())

	return cmd
}

func getCustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a custom data",
		Example: "egctl custom-data get <kind> <id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires custom data kind and id to be retrieved")
			}
			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makePath(customDataItemURL, args[0], args[1]), nil, cmd)
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
			handleRequest(http.MethodGet, makePath(customDataURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func createCustomDataCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a custom data from a yaml file or stdin",
		Example: "egctl custom-data create <kind> -f <data item file>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPost, makePath(customDataURL, args[0]), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

func updateCustomDataCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update a custom data from a yaml file or stdin",
		Example: "egctl custom-data update <kind> -f <data item file>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPut, makePath(customDataURL, args[0]), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

func batchUpdateCustomDataCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     "batch-update",
		Short:   "Batch update custom data from a yaml file or stdin",
		Example: "egctl custom-data batch-update <kind> -f <change request file>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires custom data kind to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				handleRequest(http.MethodPost, makePath(customDataItemURL, args[0], "items"), yamlDoc, cmd)
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the change request.")

	return cmd
}

func deleteCustomDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a custom data item",
		Example: "egctl custom-data delete <kind> <id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires custom data kind and id to be retrieved")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makePath(customDataItemURL, args[0], args[1]), nil, cmd)
		},
	}

	return cmd
}
