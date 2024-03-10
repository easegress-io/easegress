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
	examples := []general.Example{
		{Desc: "Print the most recent 500 logs by default.", Command: "egctl logs"},
		{Desc: "Print the most recent 100 logs.", Command: "egctl logs --tail 100"},
		{Desc: "Print all logs.", Command: "egctl logs --tail -1"},
		{Desc: "Print the most recent 500 logs and streaming the log.", Command: "egctl logs -f"},
	}

	cmd := &cobra.Command{
		Use:     "logs",
		Short:   "Print the logs of Easegress server",
		Args:    cobra.NoArgs,
		Example: createMultiExample(examples),
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
	cmd.AddCommand(setLogLevelCmd())
	cmd.AddCommand(getLogLevelCmd())
	return cmd
}

func setLogLevelCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Set log level to info", Command: "egctl logs set-level info"},
		{Desc: "Set log level to debug", Command: "egctl logs set-level debug"},
	}

	cmd := &cobra.Command{
		Use:     "set-level",
		Short:   "Set Easegress log level",
		Example: createMultiExample(examples),
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			level := args[0]
			p := general.LogsLevelURL + "/" + level
			if _, err := general.HandleRequest(http.MethodPut, p, nil); err != nil {
				general.ExitWithError(err)
			}
			fmt.Println("Set log level to", level)
		},
	}
	return cmd
}

func getLogLevelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get-level",
		Short:   "Get Easegress log level",
		Example: createExample("Get current log level.", "egctl logs get-level"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := general.HandleRequest(http.MethodGet, general.LogsLevelURL, nil)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println(string(body))
		},
	}
	return cmd
}
