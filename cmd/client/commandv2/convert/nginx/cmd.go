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

package nginx

import (
	"encoding/json"
	"fmt"

	"github.com/megaease/easegress/v2/cmd/client/general"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
	"github.com/spf13/cobra"
)

// Cmd returns convert nginx.conf command.
func Cmd() *cobra.Command {
	var nginxConf string
	cmd := &cobra.Command{
		Use:   "nginx",
		Short: "Convert nginx.conf to easegress yaml file",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := crossplane.Parse(nginxConf, &crossplane.ParseOptions{})
			if err != nil {
				general.ExitWithErrorf("parse nginx.conf failed: %v", err)
			}
			data, err := json.Marshal(payload)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println(string(data))
			general.Warnf("warn")
		},
	}
	cmd.Flags().StringVarP(&nginxConf, "file", "f", "", "nginx.conf file path")
	return cmd
}
