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
	"context"
	"errors"

	"github.com/megaease/easegress/v2/cmd/builder/build"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
)

var runConfig string

func RunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Easegress with custom plugins in current directory",
		Args:  runArgs,
		Run:   runRun,
	}
	cmd.Flags().StringVarP(&runConfig, "config-file", "f", "", "config file to run Easegress with custom plugins")
	return cmd
}

func runArgs(cmd *cobra.Command, args []string) error {
	if len(runConfig) == 0 {
		return errors.New("config file is required")
	}
	return nil
}

func runRun(cmd *cobra.Command, args []string) {
	ctx, stop := utils.WithInterrupt(context.Background())
	defer stop()

	runner, err := build.NewRunner(runConfig)
	if err != nil {
		utils.ExitWithError(err)
	}
	err = build.Run(ctx, runner)
	if err != nil {
		utils.ExitWithError(err)
	}
}
