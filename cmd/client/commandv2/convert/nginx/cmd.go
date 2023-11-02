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

package nginx

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/megaease/easegress/v2/cmd/client/general"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
	"github.com/spf13/cobra"
)

type Options struct {
	NginxConf string
	Output    string
	Prefix    string
	usedNames map[string]struct{}
}

// Cmd returns convert nginx.conf command.
func Cmd() *cobra.Command {
	flags := &Options{}
	flags.init()
	cmd := &cobra.Command{
		Use:   "nginx",
		Short: "Convert nginx.conf to easegress yaml file",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := crossplane.Parse(flags.NginxConf, &crossplane.ParseOptions{})
			if err != nil {
				general.ExitWithErrorf("parse nginx.conf failed: %v", err)
			}
			config, err := parsePayload(payload)
			if err != nil {
				general.ExitWithError(err)
			}
			hs, pls, err := convertConfig(flags, config)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println(hs, pls)
			data, err := json.Marshal(payload)
			if err != nil {
				general.ExitWithError(err)
			}
			fmt.Println(string(data))
		},
	}
	cmd.Flags().StringVarP(&flags.NginxConf, "file", "f", "", "nginx.conf file path")
	cmd.Flags().StringVarP(&flags.Output, "output", "o", "", "output yaml file path")
	cmd.Flags().StringVar(&flags.Prefix, "prefix", "", "prefix of output yaml resources")
	return cmd
}

func (opt *Options) init() {
	opt.usedNames = make(map[string]struct{})
	opt.usedNames[""] = struct{}{}
}

// GetPipelineName create a global uniq name for pipeline based on path.
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
		return fmt.Sprintf("%s-%s", opt.Prefix, name)
	}
	for i := 0; i < 8; i++ {
		nameRunes = append(nameRunes, letters[rand.Intn(len(letters))])
	}
	name = string(nameRunes)
	if _, ok := opt.usedNames[name]; !ok {
		opt.usedNames[name] = struct{}{}
		return fmt.Sprintf("%s-%s", opt.Prefix, name)
	}
	return opt.GetPipelineName(path)
}
