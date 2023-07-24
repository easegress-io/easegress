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
	"errors"
	"fmt"
	"os"

	"github.com/megaease/easegress/v2/cmd/builder/generate"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
)

var addFlags = &generate.ObjectConfig{}

func AddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add filter or resources to the project",
		Args:  addArgs,
		Run:   addRun,
	}

	cmd.Flags().StringSliceVar(&addFlags.Filters, "filters", []string{}, "filters to be generated")
	cmd.Flags().StringSliceVar(&addFlags.Resources, "resources", []string{}, "resources to be generated")
	return cmd
}

func addArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("add takes no arguments")
	}
	if (len(addFlags.Filters) == 0) && (len(addFlags.Resources) == 0) {
		return errors.New("filters or resources is required")
	}

	for _, filter := range addFlags.Filters {
		if !(utils.CapitalVariableName(filter)) {
			return fmt.Errorf("filter %s is not a valid golang variable name with first letter upper case", filter)
		}
	}
	for _, resource := range addFlags.Resources {
		if !(utils.CapitalVariableName(resource)) {
			return fmt.Errorf("resource %s is not a valid golang variable name with first letter upper case", resource)
		}
	}
	return nil
}

func addRun(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		utils.ExitWithErrorf("get current working directory failed: %s", err.Error())
	}
	config, err := generate.ReadObjectConfigFile(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	err = addCheckConfig(config)
	if err != nil {
		utils.ExitWithError(err)
	}

	// generate filters and resources dir and files.
	err = utils.MakeDirs(cwd, addFlags.Filters, addFlags.Resources)
	if err != nil {
		utils.ExitWithError(err)
	}
	for _, f := range addFlags.Filters {
		file := generate.CreateFilter(f)
		err := file.Save(utils.GetFilterFileName(cwd, f))
		if err != nil {
			utils.ExitWithErrorf("add filter %s failed: %s", f, err.Error())
		} else {
			fmt.Printf("add filter %s successfully\n", f)
		}
	}
	for _, r := range addFlags.Resources {
		file := generate.CreateResource(r)
		err := file.Save(utils.GetResourceFileName(cwd, r))
		if err != nil {
			utils.ExitWithErrorf("add resource %s failed: %s", r, err.Error())
		} else {
			fmt.Printf("add resource %s successfully\n", r)
		}
	}

	config.Filters = append(config.Filters, addFlags.Filters...)
	config.Resources = append(config.Resources, addFlags.Resources...)

	// generate registry dir and file.
	err = os.MkdirAll(utils.GetRegistryDir(cwd), os.ModePerm)
	if err != nil {
		utils.ExitWithErrorf("make directory %s failed: %s", utils.GetRegistryDir(cwd), err.Error())
	}
	file := generate.CreateRegistry(config)
	err = file.Save(utils.GetRegistryFileName(cwd))
	if err != nil {
		utils.ExitWithErrorf("update registry file failed: %s", err.Error())
	} else {
		fmt.Println("update registry file successfully")
	}
}

func addCheckConfig(config *generate.ObjectConfig) error {
	arrToSet := func(arr []string) map[string]struct{} {
		set := make(map[string]struct{})
		for _, a := range arr {
			set[a] = struct{}{}
		}
		return set
	}

	filters := arrToSet(config.Filters)
	for _, f := range addFlags.Filters {
		if _, ok := filters[f]; ok {
			return fmt.Errorf("filter %s already exists", f)
		}
	}

	resources := arrToSet(config.Resources)
	for _, r := range addFlags.Resources {
		if _, ok := resources[r]; ok {
			return fmt.Errorf("resource %s already exists", r)
		}
	}
	return nil
}
