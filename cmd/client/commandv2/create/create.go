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

// Package create provides create commands.
package create

import (
	"errors"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/spf13/cobra"
)

// Cmd returns create command.
func Cmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Create a resource from a file", Command: "egctl create -f <filename>.yaml"},
		{Desc: "Create a resource from stdin", Command: "cat <filename>.yaml | egctl create -f -"},
	}

	var specFile string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a resource from a file or from stdin.",
		Example: general.CreateMultiExample(examples),
		Args: func(cmd *cobra.Command, args []string) error {
			if specFile == "" {
				return errors.New("yaml file is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			visitor := general.BuildSpecVisitor(specFile, cmd)
			visitor.Visit(func(s *general.Spec) error {
				var err error
				defer func() {
					if err != nil {
						general.ExitWithError(err)
					}
				}()

				switch s.Kind {
				case resources.CustomDataKind().Kind:
					err = resources.CreateCustomDataKind(cmd, s)
				case resources.CustomData().Kind:
					err = resources.CreateCustomData(cmd, s)
				default:
					err = resources.CreateObject(cmd, s)
				}
				return err
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")
	return cmd
}
