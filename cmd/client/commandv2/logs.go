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

import (
	"bufio"
	"fmt"
	"io"
	"net/http"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/spf13/cobra"
)

// LogsCmd returns logs command.
func LogsCmd() *cobra.Command {
	var n int
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Print the logs of Easegress server",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			query := fmt.Sprintf("?tail=%d&follow=%v", n, follow)
			p := general.LogsURL + query
			reader, err := general.HandleReqWithStreamResp(http.MethodGet, p, nil)
			if err != nil {
				general.ExitWithError(err)
			}
			defer reader.Close()
			r := bufio.NewReader(reader)
			for {
				bytes, err := r.ReadBytes('\n')
				if err != nil {
					if err != io.EOF {
						general.ExitWithError(err)
					}
					return
				}
				fmt.Print(string(bytes))
			}
		},
	}
	cmd.Flags().IntVar(&n, "tail", 500, "Lines of recent log file to display. Defaults to 500, use -1 to show all lines")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Specify if the logs should be streamed.")
	return cmd
}
