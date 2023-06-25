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

package resources

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

const ObjectName = "object"

func ObjectAlias() []string {
	return []string{"o", "obj", "objects"}
}

const ObjectKindName = "objectkind"

func ObjectKindAlias() []string {
	return []string{"objectkinds", "ok"}
}

const ObjectTemplateName = "objecttemplate"

func ObjectTemplateAlias() []string {
	return []string{"objecttemplates", "ot"}
}

func objectCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return objectGetCmd()
	case general.DescribeCmd:
		return objectDescribeCmd()
	case general.CreateCmd:
		return objectCreateCmd()
	case general.DeleteCmd:
		return objectDeleteCmd()
	case general.ApplyCmd:
		return objectApplyCmd()
	default:
		return nil
	}
}

func objectGetCmd() []*cobra.Command {
	return []*cobra.Command{getObject(), getObjectKinds(), getObjectTemplate()}
}

// getObjectTemplate returns the object template for given object kind and name in yaml format.
// The api return yaml body to keep the right order of fields.
func getObjectTemplate() *cobra.Command {
	cmd := &cobra.Command{
		Use:     ObjectTemplateName,
		Short:   "Display object templates for given object kind and name",
		Aliases: ObjectTemplateAlias(),
		Args:    cobra.ExactArgs(2),
		Example: createExample("Get object template for given object kind and name", "egctl get objecttemplate <object-kind> <object-name>"),
		Run: func(cmd *cobra.Command, args []string) {
			url := makeURL(general.ObjectTemplateURL, args[0], args[1])
			body, err := handleReq(http.MethodGet, url, nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			}

			if general.CmdGlobalFlags.OutputFormat == general.JsonFormat {
				jsonBody, err := codectool.YAMLToJSON(body)
				if err != nil {
					general.ExitWithErrorf("yaml %s to json failed: %v", body, err)
				}
				general.PrintBody(jsonBody)
				return
			}
			fmt.Printf("%s\n", string(body))
		},
	}
	return cmd
}

func httpGetObjectArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errors.New("requires at most one arg for object name")
	}
	return nil
}

func httpGetObject(cmd *cobra.Command, args []string) ([]byte, error) {
	url := func(args []string) string {
		if len(args) == 0 {
			return makeURL(general.ObjectsURL)
		}
		return makeURL(general.ObjectItemURL, args[0])
	}(args)
	return handleReq(http.MethodGet, url, nil, cmd)
}

func getObject() *cobra.Command {
	examples := []general.Example{
		{Desc: "Get all objects", Command: "egctl get object"},
		{Desc: "Get one object", Command: "egctl get object <object-name>"},
	}

	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Display one or many objects",
		Aliases: ObjectAlias(),
		Args:    httpGetObjectArgs,
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := httpGetObject(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			metas, err := unmarshalMetaSpec(body, len(args) == 0)
			if err != nil {
				general.ExitWithErrorf("Display objects failed: %v", err)
			}
			printMetaSpec(metas)
		},
	}
	return cmd
}

func getObjectKinds() *cobra.Command {
	cmd := &cobra.Command{
		Use:     ObjectKindName,
		Short:   "Display available object kinds",
		Aliases: ObjectKindAlias(),
		Args:    cobra.ExactArgs(0),
		Example: createExample("Display available object kinds", "egctl get objectkind"),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makeURL(general.ObjectKindsURL), nil, cmd)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			kinds := []string{}
			err = codectool.Unmarshal(body, &kinds)
			if err != nil {
				general.ExitWithErrorf("Display object kinds failed: %v", err)
			}
			printObjectKinds(kinds)
		},
	}
	return cmd
}

func unmarshalMetaSpec(body []byte, listBody bool) ([]*supervisor.MetaSpec, error) {
	if listBody {
		metas := []*supervisor.MetaSpec{}
		err := codectool.Unmarshal(body, &metas)
		return metas, err
	}
	meta := &supervisor.MetaSpec{}
	err := codectool.Unmarshal(body, meta)
	return []*supervisor.MetaSpec{meta}, err
}

func getAgeFromMetaSpec(meta *supervisor.MetaSpec) string {
	createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
	if err != nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(createdAt))
}

func printMetaSpec(metas []*supervisor.MetaSpec) {
	table := [][]string{}
	table = append(table, []string{"NAME", "KIND", "AGE"})
	for _, meta := range metas {
		table = append(table, []string{meta.Name, meta.Kind, getAgeFromMetaSpec(meta)})
	}
	general.PrintTable(table)
}

func printObjectKinds(kinds []string) {
	table := [][]string{}
	table = append(table, []string{"KIND"})
	for _, kind := range kinds {
		table = append(table, []string{kind})
	}
	general.PrintTable(table)
}

func objectDescribeCmd() []*cobra.Command {
	return []*cobra.Command{describeObject()}
}

func describeObject() *cobra.Command {
	examples := []general.Example{
		{Desc: "Describe all object", Command: "egctl describe object"},
		{Desc: "Describe one object", Command: "egctl describe object <object-name>"},
	}

	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Describe one or many objects",
		Aliases: ObjectAlias(),
		Args:    httpGetObjectArgs,
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := httpGetObject(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			specs, err := general.UnmarshalMapInterface(body, len(args) == 0)
			if err != nil {
				general.ExitWithErrorf("Display objects failed: %v", err)
			}
			specials := []string{"name", "kind", "version", "", "flow", "", "filters", "", "rules", ""}
			general.PrintMapInterface(specs, specials)
		},
	}
	return cmd
}

func objectCreateCmd() []*cobra.Command {
	return []*cobra.Command{createObject()}
}

func createObject() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Create an object from a yaml file or stdin",
		Aliases: ObjectAlias(),
		Example: createExample("Create an object from a yaml file", "egctl create object -f <object>.yaml"),
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildSpecVisitor(specFile, cmd)
			visitor.Visit(func(s *spec) error {
				_, err := handleReq(http.MethodPost, makeURL(general.ObjectsURL), []byte(s.doc), cmd)
				if err != nil {
					general.ExitWithError(err)
				} else {
					fmt.Printf("Create object %s successfully\n", s.Name)
				}
				return err
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")
	return cmd
}

func objectDeleteCmd() []*cobra.Command {
	return []*cobra.Command{deleteObject()}
}

func deleteObject() *cobra.Command {
	examples := []general.Example{
		{Desc: "Delete all objects", Command: "egctl delete object --all"},
		{Desc: "Delete one object", Command: "egctl delete object <object-name>"},
	}

	var specFile string
	var allFlag bool

	argsFunc := func(cmd *cobra.Command, args []string) error {
		if allFlag {
			if (len(specFile) != 0) || (len(args) != 0) {
				return errors.New("--all cannot be used with --file or <object_name>")
			}
			return nil
		}

		if len(args) == 0 && len(specFile) == 0 {
			return errors.New("requires <object_name> or --file")
		} else if len(args) != 0 && len(specFile) != 0 {
			return errors.New("--file and <object_name> cannot be used together")
		}
		return nil
	}

	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Delete an object(s) from a yaml file or name",
		Aliases: ObjectAlias(),
		Args:    argsFunc,
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			defer func() {
				if err != nil {
					general.ExitWithError(err)
				} else {
					fmt.Println("Delete object(s) successfully")
				}
			}()

			if allFlag {
				_, err = handleReq(http.MethodDelete, makeURL(general.ObjectsURL+fmt.Sprintf("?all=%v", true)), nil, cmd)
				return
			}

			if len(specFile) != 0 {
				visitor := buildSpecVisitor(specFile, cmd)
				visitor.Visit(func(s *spec) error {
					_, err = handleReq(http.MethodDelete, makeURL(general.ObjectItemURL, s.Name), nil, cmd)
					return nil
				})
				visitor.Close()
				return
			}

			_, err = handleReq(http.MethodDelete, makeURL(general.ObjectItemURL, args[0]), nil, cmd)
		},
	}
	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")
	cmd.Flags().BoolVarP(&allFlag, "all", "", false, "Delete all object.")
	return cmd
}

func objectApplyCmd() []*cobra.Command {
	return []*cobra.Command{applyObject()}
}

func applyObject() *cobra.Command {
	var specFile string

	checkObjExist := func(cmd *cobra.Command, name string) bool {
		_, err := httpGetObject(cmd, []string{name})
		return err == nil
	}

	createOrUpdate := func(cmd *cobra.Command, s *spec, exist bool) error {
		if exist {
			_, err := handleReq(http.MethodPut, makeURL(general.ObjectItemURL, s.Name), []byte(s.doc), cmd)
			return err
		}
		_, err := handleReq(http.MethodPost, makeURL(general.ObjectsURL), []byte(s.doc), cmd)
		return err
	}

	cmd := &cobra.Command{
		Use:     ObjectName,
		Short:   "Apply a configuration to an object by filename or stdin",
		Aliases: ObjectAlias(),
		Example: createExample("Apply a configuration to an object by filename", "egctl apply object -f <object>.yaml"),
		Run: func(cmd *cobra.Command, args []string) {
			visitor := buildSpecVisitor(specFile, cmd)
			visitor.Visit(func(s *spec) error {
				exist := checkObjExist(cmd, s.Name)
				err := createOrUpdate(cmd, s, exist)

				if err != nil {
					general.ExitWithError(err)
					return err
				}

				if exist {
					fmt.Printf("Create object %s successfully\n", s.Name)
				} else {
					fmt.Printf("Update object %s successfully\n", s.Name)
				}
				return nil
			})
			visitor.Close()
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}
