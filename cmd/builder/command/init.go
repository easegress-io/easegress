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

	"github.com/megaease/easegress/cmd/builder/generate"
	"github.com/megaease/easegress/cmd/builder/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
)

var initFlags = struct {
	filters   []string
	resources []string
	repo      string
}{
	filters:   []string{},
	resources: []string{},
	repo:      "",
}

func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Init a new Easegress custom module project",
		Args:  initArgs,
		Run:   initRun,
	}

	cmd.Flags().StringSliceVar(&initFlags.filters, "filters", []string{}, "filters to be generated")
	cmd.Flags().StringSliceVar(&initFlags.resources, "resources", []string{}, "resources to be generated")
	cmd.Flags().StringVar(&initFlags.repo, "repo", "", "pkg name of the repo")
	return cmd
}

func initArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("init takes no arguments")
	}
	if len(initFlags.repo) == 0 {
		return errors.New("repo is required")
	}
	if (len(initFlags.filters) == 0) && (len(initFlags.resources) == 0) {
		return errors.New("filters or resources is required")
	}

	err := module.CheckPath(initFlags.repo)
	if err != nil {
		return fmt.Errorf("repo %s is not a valid path, %s", initFlags.repo, err.Error())
	}
	for _, filter := range initFlags.filters {
		if !(utils.CapitalVariableName(filter)) {
			return fmt.Errorf("filter %s is not a valid golang variable name with first letter upper case", filter)
		}
	}
	for _, resource := range initFlags.resources {
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

	err = utils.MakeDirs(cwd, initFlags.filters, initFlags.resources)
	if err != nil {
		utils.ExitWithError(err)
	}

	for _, f := range initFlags.filters {
		file := generate.CreateFilter(f)
		err := file.Save(utils.GetFilterFileName(cwd, f))
		if err != nil {
			utils.ExitWithErrorf("generate filter %s failed: %s", f, err.Error())
		}
	}
	for _, r := range initFlags.resources {
		file := generate.CreateResource(r)
		err := file.Save(utils.GetResourceFileName(cwd, r))
		if err != nil {
			utils.ExitWithErrorf("generate resource %s failed: %s", r, err.Error())
		}
	}
}
