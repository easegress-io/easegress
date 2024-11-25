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

// Package command contains commands for egbuilder.
package command

import (
	"errors"
	"fmt"
	"os"

	"github.com/megaease/easegress/v2/cmd/builder/gen"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/spf13/cobra"
)

var addConfig = &gen.Config{}

// AddCmd adds filter or controllers to the project.
func AddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add",
		Example: utils.CreateExample("add new custom plugins to the module", "egbuilder add --filters=MyFilter1,MyFilter2 --controllers=MyController1,MyController2"),
		Short:   "Add filter or controllers to the project",
		Args:    addArgs,
		Run:     addRun,
	}

	cmd.Flags().StringSliceVar(&addConfig.Filters, "filters", []string{}, "filters to be generated")
	cmd.Flags().StringSliceVar(&addConfig.Controllers, "controllers", []string{}, "controllers to be generated")
	return cmd
}

func addArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("add takes no arguments")
	}
	if (len(addConfig.Filters) == 0) && (len(addConfig.Controllers) == 0) {
		return errors.New("filters or controllers is required")
	}

	for _, filter := range addConfig.Filters {
		if !(utils.ExportableVariableName(filter)) {
			return fmt.Errorf("filter %s is not a valid golang variable name with first letter upper case", filter)
		}
	}
	for _, controller := range addConfig.Controllers {
		if !(utils.ExportableVariableName(controller)) {
			return fmt.Errorf("controller %s is not a valid golang variable name with first letter upper case", controller)
		}
	}
	return nil
}

func addRun(_ *cobra.Command, _ []string) {
	cwd, err := os.Getwd()
	if err != nil {
		utils.ExitWithError(err)
	}

	config := &gen.Config{}
	err = config.Load(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	err = config.CheckDuplicate(addConfig)
	if err != nil {
		utils.ExitWithError(err)
	}

	// generate filters and controllers dir and files.
	err = addConfig.GenFilters(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
	err = addConfig.GenControllers(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}

	config.Filters = append(config.Filters, addConfig.Filters...)
	config.Controllers = append(config.Controllers, addConfig.Controllers...)

	err = config.GenRegistry(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}

	err = config.Save(cwd)
	if err != nil {
		utils.ExitWithError(err)
	}
}
