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
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// AICmd returns AI command.
func AICmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Enable AI (Create the AIGatewayController)", Command: "egctl enable"},
		{Desc: "Disable AI (Delete the AIGatewayController)", Command: "egctl disable"},
		{Desc: "Get AI statistics", Command: "egctl ai stat"},
		{Desc: "Check AI health of providers", Command: "egctl ai check"},
	}

	cmd := &cobra.Command{
		Use:     "ai",
		Short:   "Commands to manage AI Gateway",
		Args:    cobra.NoArgs,
		Example: createMultiExample(examples),
		Run:     aiCmdRun,
	}

	cmd.AddCommand(
		enableCmd(),
		disableCmd(),
		statCmd(),
		checkCmd(),
		editCmd(),
	)

	return cmd
}

func aiCmdRun(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func enableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable AI Gateway (create AIGatewayController)",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			defer func() {
				if err != nil {
					general.ExitWithError(err)
				}

				fmt.Println("AI Gateway enabled successfully.")
				fmt.Println("You can use `egctl ai edit` to add providers and middlewares.")
			}()

			s, err := general.GetSpecFromYaml(`kind: AIGatewayController
name: AIGatewayController
`)
			if err != nil {
				return
			}

			err = resources.CreateObject(cmd, s)
		},
	}
}

func disableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable AI Gateway (delete AIGatewayController)",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			defer func() {
				if err != nil {
					general.ExitWithError(err)
				}
				fmt.Println("AI Gateway disabled successfully.")
			}()

			err = resources.DeleteObject(cmd, "AIGatewayController", []string{"AIGatewayController"}, false)
		},
	}
}

func statCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stat",
		Short:   "Get AI Gateway statistics",
		Example: createExample("Get AI Gateway statistics.", "egctl ai stat"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := general.HandleRequest(http.MethodGet, general.AIStatURL, nil)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			type AIStatResponse struct {
				Stats []metricshub.MetricStats `json:"stats"`
			}

			var statResp AIStatResponse
			err = codectool.UnmarshalJSON(body, &statResp)
			if err != nil {
				general.ExitWithError(err)
			}

			// Output table:
			// PROVIDER (TYPE), MODEL @ BASEURL, RESP_TYPE,
			// TOTAL_REQUESTS, SUCCESS/FAILED, AVG_DURATION(ms),
			// TOKENS (INPUT/OUTPUT)

			table := [][]string{
				{
					"PROVIDER(TYPE)",
					"MODEL@BASEURL",
					"RESP-TYPE",
					"TOTAL-REQ",
					"SUCCESS/FAILED",
					"AVG-DUR(ms)",
					"TOKENS(INPUT/OUTPUT)",
				},
			}
			for _, stat := range statResp.Stats {
				table = append(table, []string{
					fmt.Sprintf("%s(%s)", stat.Provider, stat.ProviderType),
					fmt.Sprintf("%s@%s", stat.Model, stat.BaseURL),
					stat.RespType,
					fmt.Sprintf("%d", stat.TotalRequests),
					fmt.Sprintf("%d/%d", stat.SuccessRequests, stat.FailedRequests),
					fmt.Sprintf("%d", stat.RequestAverageDuration),
					fmt.Sprintf("%d/%d", stat.PromptTokens, stat.CompletionTokens),
				})
			}
			general.PrintTable(table)
		},
	}
}

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check AI Gateway health of providers",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := general.HandleRequest(http.MethodGet, general.AISProviderstatusURL, nil)
			if err != nil {
				general.ExitWithError(err)
			}
			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			var statusResp aigatewaycontroller.HealthCheckResponse
			err = codectool.UnmarshalJSON(body, &statusResp)
			if err != nil {
				general.ExitWithError(err)
			}

			table := [][]string{
				{"NAME", "PROVIDER-TYPE", "HEALTHY"},
			}
			for _, result := range statusResp.Results {
				var healthy string
				if result.Healthy {
					healthy = "YES"
				} else {
					healthy = fmt.Sprintf("NO(%s)", result.Error)
				}

				table = append(table, []string{
					result.Name,
					result.ProviderType,
					healthy,
				})
			}
			general.PrintTable(table)
		},
	}
}

func editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit AI Gateway providers",
		Run: func(cmd *cobra.Command, args []string) {
			args = []string{"AIGatewayController", "AIGatewayController"}
			editCmdRun(cmd, args)
		},
	}
}
