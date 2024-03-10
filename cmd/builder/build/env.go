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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/cmd/builder/gen"
	"github.com/megaease/easegress/v2/cmd/builder/utils"
)

func newEnvironment(ctx context.Context, config *Config) (*environment, error) {
	// create main.go file to run easegress
	plugins := []string{}
	for _, p := range config.Plugins {
		plugins = append(plugins, p.Module)
	}
	file := gen.CreateServer(plugins)

	// create temporary dir
	tempDir, err := newTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err2 := os.RemoveAll(tempDir)
			if err2 != nil {
				err = fmt.Errorf("%w; clean up temp dir failed: %v", err, err2)
			}
		}
	}()
	fmt.Printf("Temporary folder for building: %s\n", tempDir)

	err = file.Save(filepath.Join(tempDir, "main.go"))
	if err != nil {
		return nil, fmt.Errorf("save main.go failed: %v", err)
	}

	env := &environment{
		config:  config,
		tempDir: tempDir,
	}

	cmd := env.newGoCmdWithModFlags(ctx, "mod", "init")
	cmd.Args = append(cmd.Args, "easegress-server")
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %v", strings.Join(cmd.Args, " "), err)
	}

	// specify module replacements before pinning versions
	replaced := make(map[string]string)
	for _, plugin := range config.Plugins {
		if plugin.Replacement == "" {
			continue
		}
		cmd := env.newGoCmdWithModFlags(ctx, "mod", "edit", "-replace", fmt.Sprintf("%s=%s", plugin.Module, plugin.Replacement))
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("%s failed: %v", strings.Join(cmd.Args, " "), err)
		}
		replaced[plugin.Module] = plugin.Replacement
	}

	// check for early abort
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	err = env.execGoGet(ctx, utils.EGPath, env.config.EGVersion, "", "")
	if err != nil {
		return nil, err
	}

	for _, p := range config.Plugins {
		_, ok := replaced[p.Module]
		if ok {
			continue
		}
		// also pass the Easegress version to prevent it from being upgraded
		err = env.execGoGet(ctx, p.Module, p.Version, utils.EGPath, env.config.EGVersion)
		if err != nil {
			return nil, err
		}

		// check for early abort
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	// doing an empty "go get -d" can potentially resolve some
	// ambiguities introduced by one of the plugins;
	err = env.execGoGet(ctx, "", "", "", "")
	if err != nil {
		return nil, err
	}
	return env, nil
}

type environment struct {
	config  *Config
	tempDir string
}

// Close cleans up the build environment, including deleting
// the temporary folder from the disk.
func (env *environment) Close() error {
	if env.config.SkipCleanup {
		fmt.Printf("skipCleanup is true, temporary folder: %s\n", env.tempDir)
		return nil
	}
	fmt.Printf("clean up temporary dir: %s", env.tempDir)
	return os.RemoveAll(env.tempDir)
}

func (env *environment) newGoCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := utils.NewExecCmd(ctx, utils.GetGo(), args...)
	cmd.Dir = env.tempDir
	return cmd
}

func (env *environment) newGoCmdWithModFlags(ctx context.Context, args ...string) *exec.Cmd {
	cmd := env.newGoCmd(ctx, args...)
	cmd.Args = append(cmd.Args, env.config.ModFlags...)
	return cmd
}

func (env *environment) newGoCmdWithBuildFlags(ctx context.Context, args ...string) *exec.Cmd {
	cmd := env.newGoCmd(ctx, args...)
	cmd.Args = append(cmd.Args, env.config.BuildFlags...)
	return cmd
}

// execGoGet runs the command "go get -d -v" with the provided module/version as an argument.
// It also supports passing a secondary module/version pair, which is typically the main Easegress
// module/version that the build is against. This prevents the plugin module from triggering an
// upgrade of the Easegress version, which could occur if the plugin version requires a newer Easegress version。
func (env environment) execGoGet(ctx context.Context, modulePath, moduleVersion, egPath, egVersion string) error {
	mod := modulePath
	if moduleVersion != "" {
		mod += "@" + moduleVersion
	}
	eg := egPath
	if egVersion != "" {
		eg += "@" + egVersion
	}

	cmd := env.newGoCmdWithModFlags(ctx, "get", "-d", "-v")
	// using an empty string as an additional argument to "go get"
	// breaks the command since it treats the empty string as a
	// distinct argument, so we're using an if statement to avoid it.
	if eg != "" {
		cmd.Args = append(cmd.Args, mod, eg)
	} else {
		cmd.Args = append(cmd.Args, mod)
	}
	return cmd.Run()
}

// newTempDir creates a new dir in a temporary location.
// It is the caller's responsibility to remove the folder when finished.
func newTempDir() (string, error) {
	parentDir := ""
	if runtime.GOOS == "darwin" {
		// go build -ldflags` inside of $TMPDIR on macOS High Sierra is broken.
		// and https://twitter.com/mholt6/status/978345803365273600 (thread)
		// (using an absolute path prevents problems later when removing this
		// folder if the CWD changes)
		var err error
		parentDir, err = filepath.Abs(".")
		if err != nil {
			return "", err
		}
	}
	ts := time.Now().Format("2006-01-02-1504")
	return os.MkdirTemp(parentDir, fmt.Sprintf("egbuilder_%s.", ts))
}
