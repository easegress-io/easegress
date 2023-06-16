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

package commandv2

// func ApiResourceCmd() *cobra.Command {
// 	cmd := &cobra.Command{
// 		Use:   "api-resources",
// 		Short: "Print the supported API resources",
// 		Args:  cobra.NoArgs,
// 		Run: func(cmd *cobra.Command, args []string) {
// 			infos := resources.Infos()

// 			if !general.CmdGlobalFlags.DefaultFormat() {
// 				data, err := codectool.MarshalJSON(infos)
// 				if err != nil {
// 					general.ExitWithErrorf("marshal API resources failed: %v", err)
// 					return
// 				}
// 				general.PrintBody(data)
// 				return
// 			}

// 			table := [][]string{}
// 			table = append(table, []string{"NAME", "ALIAS"})
// 			for _, info := range infos {
// 				table = append(table, []string{info.Name, strings.Join(info.Alias, ",")})
// 			}
// 			general.PrintTable(table)
// 		},
// 	}
// 	return cmd
// }
