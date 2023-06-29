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

package resources

// // WasmDataName is the name of the WasmData resource.
// const WasmDataName = "wasmdata"

// // WasmDataAlias is the alias of the WasmData resource.
// func WasmDataAlias() []string {
// 	return []string{"wd", "wasmdatas"}
// }

// func wasmDataCmd(cmdType general.CmdType) []*cobra.Command {
// 	switch cmdType {
// 	case general.GetCmd:
// 		return wasmDataGetCmd()
// 	case general.DeleteCmd:
// 		return wasmDataDeleteCmd()
// 	case general.ApplyCmd:
// 		return wasmDataApplyCmd()
// 	default:
// 		return nil
// 	}
// }

// func wasmDataGetCmd() []*cobra.Command {
// 	return []*cobra.Command{getWasmDataCmd()}
// }

// func getWasmDataCmd() *cobra.Command {
// 	cmd := &cobra.Command{
// 		Use:     WasmDataName,
// 		Aliases: WasmDataAlias(),
// 		Short:   "Get shared data of a WasmHost filter",
// 		Example: createExample("List shared data of a WasmHost filter ", "egctl get wasmdata <pipeline> <filter>"),
// 		Args: func(cmd *cobra.Command, args []string) error {
// 			if len(args) == 2 {
// 				return nil
// 			}
// 			return fmt.Errorf("requires pipeline and filter name")
// 		},

// 		Run: func(cmd *cobra.Command, args []string) {
// 			body, err := handleReq(http.MethodGet, makeURL(general.WasmDataURL, args[0], args[1]), nil, cmd)
// 			if err != nil {
// 				general.ExitWithError(err)
// 			}
// 			general.PrintBody(body)
// 		},
// 	}
// 	return cmd
// }

// func wasmDataDeleteCmd() []*cobra.Command {
// 	return []*cobra.Command{deleteWasmDataCmd()}
// }

// func deleteWasmDataCmd() *cobra.Command {
// 	cmd := &cobra.Command{
// 		Use:     WasmDataName,
// 		Aliases: WasmDataAlias(),
// 		Short:   "Delete all shared data of a WasmHost filter",
// 		Example: createExample("Delete all shared data of a WasmHost filter", "egctl delete wasmdata <pipeline> <filter>"),
// 		Args: func(cmd *cobra.Command, args []string) error {
// 			if len(args) == 2 {
// 				return nil
// 			}
// 			return fmt.Errorf("requires pipeline and filter name")
// 		},

// 		Run: func(cmd *cobra.Command, args []string) {
// 			body, err := handleReq(http.MethodDelete, makeURL(general.WasmDataURL, args[0], args[1]), nil, cmd)
// 			if err != nil {
// 				general.ExitWithError(err)
// 			}
// 			general.PrintBody(body)
// 		},
// 	}
// 	return cmd
// }

// func wasmDataApplyCmd() []*cobra.Command {
// 	return []*cobra.Command{applyWasmDataCmd()}
// }

// func applyWasmDataCmd() *cobra.Command {
// 	examples := []general.Example{
// 		{Desc: "Apply shared data to a WasmHost filter", Command: "egctl apply wasmdata <pipeline> <filter> -f <YAML file>"},
// 		{Desc: "Notify Easegress to reload WebAssembly code", Command: "egctl apply wasmdata --reload-code"},
// 	}

// 	var specFile string
// 	var reload bool

// 	cmd := &cobra.Command{
// 		Use:     WasmDataName,
// 		Aliases: WasmDataAlias(),
// 		Short:   "Apply shared data to a WasmHost filter",
// 		Example: createMultiExample(examples),
// 		Args: func(cmd *cobra.Command, args []string) error {
// 			if reload {
// 				if len(args) != 0 {
// 					return fmt.Errorf("reload-code does not require any arguments")
// 				}
// 				return nil
// 			}

// 			if len(args) == 2 {
// 				return nil
// 			}
// 			return fmt.Errorf("requires pipeline and filter name")
// 		},

// 		Run: func(cmd *cobra.Command, args []string) {
// 			if reload {
// 				body, err := handleReq(http.MethodPost, makeURL(general.WasmCodeURL), nil, cmd)
// 				if err != nil {
// 					general.ExitWithError(err)
// 				}
// 				general.PrintBody(body)
// 				return
// 			}

// 			visitor := buildYAMLVisitor(specFile, cmd)
// 			visitor.Visit(func(yamlDoc []byte) error {
// 				body, err := handleReq(http.MethodPut, makeURL(general.WasmDataURL, args[0], args[1]), yamlDoc, cmd)
// 				if err != nil {
// 					general.ExitWithError(err)
// 				}
// 				general.PrintBody(body)
// 				return err
// 			})
// 			visitor.Close()
// 		},
// 	}
// 	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")
// 	cmd.Flags().BoolVar(&reload, "reload-code", false, "Notify Easegress to reload WebAssembly code")
// 	return cmd
// }
