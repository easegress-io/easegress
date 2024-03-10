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
	"fmt"
	"net/http"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/spf13/cobra"
)

// WasmCmd defines member command.
func WasmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wasm",
		Short: "Manage WebAssembly code and data",
	}

	cmd.AddCommand(wasmReloadCodeCmd())
	cmd.AddCommand(wasmDeleteDataCmd())
	cmd.AddCommand(wasmApplyDataCmd())
	cmd.AddCommand(wasmListDataCmd())
	return cmd
}

func wasmReloadCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reload-code",
		Short:   "Notify Easegress to reload WebAssembly code",
		Example: createExample("Notify Easegress to reload WebAssembly code", "egctl wasm reload-code"),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodPost, makePath(general.WasmCodeURL), nil)
			if err != nil {
				general.ExitWithError(err)
			}
			general.PrintBody(body)
		},
	}

	return cmd
}

func wasmDeleteDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete-data",
		Short:   "Delete all shared data of a WasmHost filter",
		Example: createExample("Delete all shared data of a WasmHost filter", "egctl wasm delete-data <pipeline> <filter>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				return nil
			}
			return fmt.Errorf("requires pipeline and filter name")
		},

		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodDelete, makePath(general.WasmDataURL, args[0], args[1]), nil)
			if err != nil {
				general.ExitWithError(err)
			}
			general.PrintBody(body)
		},
	}

	return cmd
}

func wasmApplyDataCmd() *cobra.Command {
	var specFile string

	cmd := &cobra.Command{
		Use:     "apply-data",
		Short:   "Apply shared data to a WasmHost filter",
		Example: createExample("Apply shared data to a WasmHost filter", "egctl wasm apply-data <pipeline> <filter> -f <spec file>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				return nil
			}
			return fmt.Errorf("requires pipeline and filter name")
		},

		Run: func(cmd *cobra.Command, args []string) {
			visitor := general.BuildYAMLVisitor(specFile, cmd)
			visitor.Visit(func(yamlDoc []byte) error {
				body, err := handleReq(http.MethodPut, makePath(general.WasmDataURL, args[0], args[1]), yamlDoc)
				if err != nil {
					general.ExitWithError(err)
				}
				general.PrintBody(body)
				return nil
			})
			visitor.Close()
		},
	}
	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}

func wasmListDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-data",
		Short:   "List shared data of a WasmHost filter",
		Example: createExample("List shared data of a WasmHost filter", "egctl wasm list-data <pipeline> <filter>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				return nil
			}
			return fmt.Errorf("requires pipeline and filter name")
		},

		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makePath(general.WasmDataURL, args[0], args[1]), nil)
			if err != nil {
				general.ExitWithError(err)
			}
			general.PrintBody(body)
		},
	}

	return cmd
}
