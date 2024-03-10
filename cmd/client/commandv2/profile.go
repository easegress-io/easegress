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
	"errors"
	"net/http"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/spf13/cobra"
)

// ProfileCmd returns profile command.
func ProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Start and stop CPU and memory profilers",
	}
	cmd.AddCommand(infoProfileCmd())
	cmd.AddCommand(startProfilingCmd())
	cmd.AddCommand(stopProfilingCmd())
	return cmd
}

func infoProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info",
		Short:   "Show memory and CPU profile file paths",
		Example: createExample("Show profile info.", "egctl profile info"),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makePath(general.ProfileURL), nil)
			if err != nil {
				general.ExitWithError(err)
				return
			}
			general.PrintBody(body)
		},
	}
	return cmd
}

func startProfilingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Prepare measuring CPU or memory",
	}
	cmd.AddCommand(startCPUCmd())
	cmd.AddCommand(startMemoryCmd())

	return cmd
}

func startCPUCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cpu",
		Short:   "Prepare measuring CPU",
		Example: createExample("Start profile CPU.", "egctl profile start cpu <path/to/cpu-prof-file>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one file path")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			body := []byte("path: " + args[0])
			body, err := handleReq(http.MethodPost, makePath(general.ProfileStartURL, "cpu"), body)
			if err != nil {
				general.ExitWithError(err)
				return
			}
			if len(body) == 0 {
				body = []byte(`{msg: "CPU profiling started"}`)
			}
			general.PrintBody(body)
		},
	}
	return cmd
}

func startMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memory",
		Short:   "Prepare measuring memory",
		Example: createExample("Start profile memory.", "egctl profile start memory <path/to/memory-prof-file>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one file path")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			body := []byte("path: " + args[0])
			body, err := handleReq(http.MethodPost, makePath(general.ProfileStartURL, "memory"), body)
			if err != nil {
				general.ExitWithError(err)
				return
			}
			if len(body) == 0 {
				body = []byte(`{msg: "Memory profiling started"}`)
			}
			general.PrintBody(body)
		},
	}
	return cmd
}

func stopProfilingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop profile.",
		Example: createExample("Stop profiling", "egctl profile stop"),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodPost, makePath(general.ProfileStopURL), nil)
			if err != nil {
				general.ExitWithError(err)
				return
			}
			if len(body) == 0 {
				body = []byte(`{msg: "Profiling stopped"}`)
			}
			general.PrintBody(body)
		},
	}
	return cmd
}
