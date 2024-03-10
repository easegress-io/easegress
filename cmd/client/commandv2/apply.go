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

// Package commandv2 provides the new version of commands.
package commandv2

import (
	"errors"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/spf13/cobra"
)

// ApplyCmd returns apply command.
func ApplyCmd() *cobra.Command {
	examples := []general.Example{
		{Desc: "Apply a configuration to a resource by filename", Command: "egctl apply -f <filename>.yaml"},
		{Desc: "Apply a configuration to a resource from stdin", Command: "cat <filename>.yaml | egctl apply -f -"},
	}

	var specFile string
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Apply a configuration to a resource by filename or stdin",
		Example: createMultiExample(examples),
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
					err = resources.ApplyCustomDataKind(cmd, s)
				case resources.CustomData().Kind:
					err = resources.ApplyCustomData(cmd, s)
				default:
					err = resources.ApplyObject(cmd, s)
				}
				return err
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")
	return cmd
}
