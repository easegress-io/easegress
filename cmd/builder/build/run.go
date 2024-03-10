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

package build

import (
	"context"
	"fmt"
	"os"

	"github.com/megaease/easegress/v2/cmd/builder/gen"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

// Runner is the runner of easegress-server. It is used to build and run easegress-server.
type Runner struct {
	Options      `json:",inline"`
	EgServerArgs []string `json:"egServerArgs"`

	config *Config
}

// NewRunner creates a new runner.
func NewRunner(filename string) (*Runner, error) {
	runner := &Runner{}

	if len(filename) > 0 {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read file %s failed: %v", filename, err)
		}
		err = codectool.UnmarshalYAML(data, runner)
		if err != nil {
			return nil, fmt.Errorf("unmarshal yaml file %s failed: %v", filename, err)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get current working directory failed: %v", err)
	}

	genConfig := &gen.Config{}
	err = genConfig.Load(cwd)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	config.Options = runner.Options
	config.Plugins = []*Plugin{
		{
			Module:      genConfig.Repo,
			Replacement: cwd,
		},
	}
	err = config.Init()
	if err != nil {
		return nil, err
	}
	runner.config = config
	return runner, nil
}

// Run runs the easegress-server with plugins in current directory.
func Run(ctx context.Context, runner *Runner) error {
	config := runner.config
	err := Build(ctx, config)
	if err != nil {
		return err
	}

	cmd := utils.NewExecCmd(ctx, config.Output, runner.EgServerArgs...)
	cmd.Stdin = os.Stdin
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("start egserver failed: %v", err)
	}
	defer func() {
		if runner.SkipCleanup {
			fmt.Printf("skip cleanup, please stop egserver manually\n")
			return
		}
		err = os.Remove(config.Output)
		if err != nil && !os.IsNotExist(err) {
			fmt.Printf("deleting temporary binary %s failed: %v", config.Output, err)
		}
	}()
	return cmd.Wait()
}
