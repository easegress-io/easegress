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
	"fmt"
	"os"

	"github.com/megaease/easegress/v2/cmd/builder/gen"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
)

var initConfig = &gen.Config{}

// InitCmd creates the init command of egbuilder.
func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Init a new Easegress custom module project",
		Example: utils.CreateExample("Init a new Easegress custom module project", "egbuilder init --repo github.com/my/repo --filters=MyFilter1,MyFilter2 --controllers=MyController1,MyController2"),
		Args:    initArgs,
		Run:     initRun,
	}

	cmd.Flags().StringSliceVar(&initConfig.Filters, "filters", []string{}, "filters to be generated")
	cmd.Flags().StringSliceVar(&initConfig.Controllers, "controllers", []string{}, "controllers to be generated")
	cmd.Flags().StringVar(&initConfig.Repo, "repo", "", "pkg name of the repo")
	return cmd
}

func initArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("init takes no arguments")
	}
	if len(initConfig.Repo) == 0 {
		return errors.New("repo is required")
	}
	if (len(initConfig.Filters) == 0) && (len(initConfig.Controllers) == 0) {
		return errors.New("filters or controllers is required")
	}

	err := module.CheckPath(initConfig.Repo)
	if err != nil {
		return fmt.Errorf("repo %s is not a valid path, %s", initConfig.Repo, err.Error())
	}
	for _, filter := range initConfig.Filters {
		if !(utils.ExportableVariableName(filter)) {
			return fmt.Errorf("filter %s is not a valid golang variable name with first letter upper case", filter)
		}
	}
	for _, controller := range initConfig.Controllers {
		if !(utils.ExportableVariableName(controller)) {
			return fmt.Errorf("controller %s is not a valid golang variable name with first letter upper case", controller)
		}
	}
	return nil
}

func initRun(cmd *cobra.Command, args []string) {
	ctx, stop := utils.WithInterrupt(context.Background())
	defer stop()

	cwd, err := os.Getwd()
	if err != nil {
		utils.ExitWithError(err)
	}
	files, err := os.ReadDir(cwd)
	if err != nil {
		utils.ExitWithErrorf("read directory %s failed: %s", cwd, err.Error())
	}
	if len(files) != 0 {
		utils.ExitWithErrorf("directory %s is not empty, please call init in an empty directory", cwd)
	}

	initGenerateFiles(cmd, cwd)
	initGenerateMod(ctx, cmd)
	err = initConfig.Save(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
}

func initGenerateFiles(_ *cobra.Command, cwd string) {
	err := initConfig.GenFilters(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	fmt.Println("generate filters success")

	err = initConfig.GenControllers(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	fmt.Println("generate controllers success")

	err = initConfig.GenRegistry(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	fmt.Println("generate registry success")
}

func initGenerateMod(ctx context.Context, cmd *cobra.Command) {
	// go mod init
	modInitCmd := utils.NewExecCmd(ctx, utils.GetGo(), "mod", "init", initConfig.Repo)
	err := modInitCmd.Run()
	if err != nil {
		utils.ExitWithErrorf("exec %v failed, %v", cmd.Args, err)
	}

	// go get easegress
	modGetCmd := utils.NewExecCmd(ctx, utils.GetGo(), "get", utils.EGPath)
	err = modGetCmd.Run()
	if err != nil {
		utils.ExitWithErrorf("exec %v failed, %v", cmd.Args, err)
	}

	// go mod tidy
	modTidyCmd := utils.NewExecCmd(ctx, utils.GetGo(), "mod", "tidy")
	err = modTidyCmd.Run()
	if err != nil {
		utils.ExitWithErrorf("exec %v failed, %v", cmd.Args, err)
	}
}
