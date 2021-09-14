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

// ObjectCmd defines object command.
func ObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "object",
		Aliases: []string{"o", "obj"},
		Short:   "View and change objects",
	}

	cmd.AddCommand(objectKindsCmd())
	cmd.AddCommand(listObjectsCmd())
	cmd.AddCommand(getObjectCmd())
	cmd.AddCommand(createObjectCmd())
	cmd.AddCommand(updateObjectCmd())
	cmd.AddCommand(deleteObjectCmd())
	cmd.AddCommand(statusObjectCmd())

	return cmd
}

func objectKindsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kinds",
		Short: "List available object kinds.",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectKindsURL), nil, cmd)
		},
	}

	return cmd
}

func createObjectCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an object from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildVisitorFromFileOrStdin(specFile, cmd)
			visitor.Visit(func(s *spec) {
				handleRequest(http.MethodPost, makeURL(objectsURL), []byte(s.doc), cmd)
			})
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}

func updateObjectCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an object from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildVisitorFromFileOrStdin(specFile, cmd)
			visitor.Visit(func(s *spec) {
				handleRequest(http.MethodPut, makeURL(objectURL, s.Name), []byte(s.doc), cmd)
			})
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}

func deleteObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an object",
		Example: "egctl object delete <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(objectURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an object",
		Example: "egctl object get <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listObjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all objects",
		Example: "egctl object list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectsURL), nil, cmd)
		},
	}

	return cmd
}

func statusObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"stat"},
		Short:   "View status of object",
	}

	cmd.AddCommand(getStatusObjectCmd())
	cmd.AddCommand(listStatusObjectsCmd())

	return cmd
}

func getStatusObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get status of an object",
		Example: "egctl object status get <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(statusObjectURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listStatusObjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all status of objects",
		Example: "egctl object status list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(statusObjectsURL), nil, cmd)
		},
	}

	return cmd
}
