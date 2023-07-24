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

// Package command contains commands for egbuilder.
package command

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/megaease/easegress/v2/cmd/builder/generate"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
)

var initFlags = &generate.ObjectConfig{}

func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Init a new Easegress custom module project",
		Args:  initArgs,
		Run:   initRun,
	}

	cmd.Flags().StringSliceVar(&initFlags.Filters, "filters", []string{}, "filters to be generated")
	cmd.Flags().StringSliceVar(&initFlags.Resources, "resources", []string{}, "resources to be generated")
	cmd.Flags().StringVar(&initFlags.Repo, "repo", "", "pkg name of the repo")
	return cmd
}

func initArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("init takes no arguments")
	}
	if len(initFlags.Repo) == 0 {
		return errors.New("repo is required")
	}
	if (len(initFlags.Filters) == 0) && (len(initFlags.Resources) == 0) {
		return errors.New("filters or resources is required")
	}

	err := module.CheckPath(initFlags.Repo)
	if err != nil {
		return fmt.Errorf("repo %s is not a valid path, %s", initFlags.Repo, err.Error())
	}
	for _, filter := range initFlags.Filters {
		if !(utils.CapitalVariableName(filter)) {
			return fmt.Errorf("filter %s is not a valid golang variable name with first letter upper case", filter)
		}
	}
	for _, resource := range initFlags.Resources {
		if !(utils.CapitalVariableName(resource)) {
			return fmt.Errorf("resource %s is not a valid golang variable name with first letter upper case", resource)
		}
	}
	return nil
}

func initRun(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		utils.ExitWithErrorf("get current working directory failed: %s", err.Error())
	}
	files, err := os.ReadDir(cwd)
	if err != nil {
		utils.ExitWithErrorf("read directory %s failed: %s", cwd, err.Error())
	}
	if len(files) != 0 {
		utils.ExitWithErrorf("directory %s is not empty, please call init in an empty directory", cwd)
	}

	initGenerateFiles(cmd, cwd)
	initGenerateMod(cmd)
	generate.WriteObjectConfigFile(cwd, initFlags)
}

func initGenerateFiles(_ *cobra.Command, cwd string) {
	// generate filters and resources dir and files.
	err := utils.MakeDirs(cwd, initFlags.Filters, initFlags.Resources)
	if err != nil {
		utils.ExitWithError(err)
	}

	for _, f := range initFlags.Filters {
		file := generate.CreateFilter(f)
		err := file.Save(utils.GetFilterFileName(cwd, f))
		if err != nil {
			utils.ExitWithErrorf("generate filter %s failed: %s", f, err.Error())
		} else {
			fmt.Printf("generate filter %s success\n", f)
		}
	}
	for _, r := range initFlags.Resources {
		file := generate.CreateResource(r)
		err := file.Save(utils.GetResourceFileName(cwd, r))
		if err != nil {
			utils.ExitWithErrorf("generate resource %s failed: %s", r, err.Error())
		} else {
			fmt.Printf("generate resource %s success\n", r)
		}
	}

	// generate registry dir and file.
	err = os.MkdirAll(utils.GetRegistryDir(cwd), os.ModePerm)
	if err != nil {
		utils.ExitWithErrorf("make directory %s failed: %s", utils.GetRegistryDir(cwd), err.Error())
	}
	file := generate.CreateRegistry(initFlags)
	err = file.Save(utils.GetRegistryFileName(cwd))
	if err != nil {
		utils.ExitWithErrorf("generate registry file failed: %s", err.Error())
	} else {
		fmt.Printf("generate registry file success\n")
	}
}

func initGenerateMod(cmd *cobra.Command) {
	// go mod init
	modInitCmd := exec.Command(utils.GetGo(), "mod", "init", initFlags.Repo)
	modInitCmd.Stderr = os.Stderr
	out, err := modInitCmd.Output()
	if err != nil {
		utils.ExitWithErrorf("exec %v: %v: %s", cmd.Args, err, string(out))
	}

	// go get easegress
	modGetCmd := exec.Command(utils.GetGo(), "get", utils.EG)
	modGetCmd.Stderr = os.Stderr
	out, err = modGetCmd.Output()
	if err != nil {
		utils.ExitWithErrorf("exec %v: %v: %s", cmd.Args, err, string(out))
	}

	// go mod tidy
	modTidyCmd := exec.Command(utils.GetGo(), "mod", "tidy")
	modTidyCmd.Stderr = os.Stderr
	out, err = modTidyCmd.Output()
	if err != nil {
		utils.ExitWithErrorf("exec %v: %v: %s", cmd.Args, err, string(out))
	}
}
