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

package nginx

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/specs"
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
	"github.com/spf13/cobra"
)

// Options contains the options for convert nginx.conf.
type Options struct {
	NginxConf      string
	Output         string
	ResourcePrefix string
	usedNames      map[string]struct{}
}

// Cmd returns convert nginx.conf command.
func Cmd() *cobra.Command {
	flags := &Options{}
	flags.init()
	examples := []general.Example{
		{
			Desc:    "Convert nginx config to easegress yamls",
			Command: "egctl convert nginx -f <nginx.conf> -o <output.yaml> --resource-prefix <prefix>",
		},
	}
	cmd := &cobra.Command{
		Use:     "nginx",
		Short:   "Convert nginx.conf to easegress yaml file",
		Example: general.CreateMultiExample(examples),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.NginxConf == "" {
				return fmt.Errorf("nginx.conf file path is required")
			}
			if flags.Output == "" {
				return fmt.Errorf("output yaml file path is required")
			}
			if flags.ResourcePrefix == "" {
				return fmt.Errorf("prefix is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := crossplane.Parse(flags.NginxConf, &crossplane.ParseOptions{})
			if err != nil {
				general.ExitWithErrorf("parse nginx.conf failed: %v", err)
			}
			for _, e := range payload.Errors {
				general.Warnf("parse nginx.conf error: %v in %s of %s", e.Error, e.Line, e.File)
			}
			config, err := parsePayload(payload)
			if err != nil {
				general.ExitWithError(err)
			}
			hs, pls, err := convertConfig(flags, config)
			if err != nil {
				general.ExitWithError(err)
			}
			if err := writeYaml(flags.Output, hs, pls); err != nil {
				general.ExitWithError(err)
			}
		},
	}
	cmd.Flags().StringVarP(&flags.NginxConf, "file", "f", "", "nginx.conf file path")
	cmd.Flags().StringVarP(&flags.Output, "output", "o", "", "output yaml file path")
	cmd.Flags().StringVar(&flags.ResourcePrefix, "resource-prefix", "nginx", "prefix of output yaml resources")
	return cmd
}

func (opt *Options) init() {
	opt.usedNames = make(map[string]struct{})
	opt.usedNames[""] = struct{}{}
}

// GetPipelineName creates a globally unique name for the pipeline based on the path.
func (opt *Options) GetPipelineName(path string) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	nameRunes := make([]rune, 0)
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			nameRunes = append(nameRunes, r)
		}
	}
	name := string(nameRunes)
	if _, ok := opt.usedNames[name]; !ok {
		opt.usedNames[name] = struct{}{}
		return fmt.Sprintf("%s-%s", opt.ResourcePrefix, name)
	}
	for i := 0; i < 8; i++ {
		nameRunes = append(nameRunes, letters[rand.Intn(len(letters))])
	}
	name = string(nameRunes)
	if _, ok := opt.usedNames[name]; !ok {
		opt.usedNames[name] = struct{}{}
		return fmt.Sprintf("%s-%s", opt.ResourcePrefix, name)
	}
	return opt.GetPipelineName(path)
}

func writeYaml(filename string, servers []*specs.HTTPServerSpec, pipelines []*specs.PipelineSpec) error {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	file, err := os.Create(absPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, s := range servers {
		data, err := codectool.MarshalYAML(s)
		if err != nil {
			return err
		}
		file.WriteString(string(data))
		file.WriteString("\n---\n")
	}
	for _, p := range pipelines {
		data, err := codectool.MarshalYAML(p)
		if err != nil {
			return err
		}
		file.WriteString(string(data))
		file.WriteString("\n---\n")
	}
	file.Sync()
	return nil
}
