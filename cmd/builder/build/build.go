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

// Package build contains utils for egbuilder build command.
package build

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Build builds Easegress with custom plugins.
func Build(ctx context.Context, config *Config) error {
	// prepare the build environment
	fmt.Println("Preparing build environment...")
	buildEnv, err := newEnvironment(ctx, config)
	if err != nil {
		return err
	}
	defer buildEnv.Close()
	fmt.Println("Build environment is ready.")

	if config.SkipBuild {
		fmt.Println("Flag skipBuild is set, skip building easegress-server.")
		return nil
	}

	// prepare the environment for the go command; for
	// the most part we want it to inherit our current
	// environment, with a few customizations
	env := os.Environ()
	compile := config.Compile
	env = setEnv(env, "GOOS="+compile.OS)
	env = setEnv(env, "GOARCH="+compile.Arch)
	env = setEnv(env, "GOARM="+compile.ARM)
	if config.RaceDetector && !config.Compile.Cgo {
		fmt.Println("enabling cgo because it is required by the race detector")
		config.Compile.Cgo = true
	}
	env = setEnv(env, fmt.Sprintf("CGO_ENABLED=%s", config.Compile.CgoEnabled()))

	fmt.Println("Building easegress-server...")
	// // tidy the module to ensure go.mod and go.sum are consistent with the module prereq
	tidyCmd := buildEnv.newGoCmdWithModFlags(ctx, "mod", "tidy", "-e")
	if err := tidyCmd.Run(); err != nil {
		return err
	}

	// compile
	cmd := buildEnv.newGoCmdWithBuildFlags(ctx, "build", "-o", config.Output)
	if len(config.BuildFlags) == 0 {
		cmd.Args = append(cmd.Args,
			"-ldflags", "-w -s",
			"-trimpath",
		)
	}

	if config.RaceDetector {
		cmd.Args = append(cmd.Args, "-race")
	}
	cmd.Env = env
	err = cmd.Run()
	if err != nil {
		return err
	}
	fmt.Printf("Build complete: %s\n", config.Output)
	return nil
}

// setEnv sets an environment variable-value pair in
// env, overriding an existing variable if it already
// exists. The env slice is one such as is returned
// by os.Environ(), and set must also have the form
// of key=value.
func setEnv(env []string, set string) []string {
	parts := strings.SplitN(set, "=", 2)
	key := parts[0]
	for i := 0; i < len(env); i++ {
		if strings.HasPrefix(env[i], key+"=") {
			env[i] = set
			return env
		}
	}
	return append(env, set)
}
