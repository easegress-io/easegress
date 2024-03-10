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

package command

import (
	"context"
	"errors"

	"github.com/megaease/easegress/v2/cmd/builder/build"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
)

var buildConfig string

// BuildCmd builds Easegress with custom plugins.
func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "build",
		Example: utils.CreateExample("Build easegress-server with custom plugins", "egbuilder build -f your-build-config.yaml"),
		Short:   "Build Easegress with custom plugins",
		Args:    buildArgs,
		Run:     buildRun,
	}
	cmd.Flags().StringVarP(&buildConfig, "config-file", "f", "", "config file to build Easegress with custom plugins")
	return cmd
}

func buildArgs(_ *cobra.Command, _ []string) error {
	if len(buildConfig) == 0 {
		return errors.New("config file is required")
	}
	return nil
}

func buildRun(_ *cobra.Command, _ []string) {
	ctx, stop := utils.WithInterrupt(context.Background())
	defer stop()

	config, err := build.NewConfig(buildConfig)
	if err != nil {
		utils.ExitWithError(err)
	}
	err = build.Build(ctx, config)
	if err != nil {
		utils.ExitWithError(err)
	}
}
