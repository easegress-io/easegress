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

package generate

import (
	"fmt"
	"os"
	"path"

	j "github.com/dave/jennifer/jen"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	objectConfigFileName = "egbuilder-config.yaml"
	KindName             = "Kind"
	SpecName             = "Spec"
	apiName              = "API"
	yourCodeHere         = "your code here\n"

	egFilters    = "github.com/megaease/easegress/v2/pkg/filters"
	egContext    = "github.com/megaease/easegress/v2/pkg/context"
	egSupervisor = "github.com/megaease/easegress/v2/pkg/supervisor"
	egAPI        = "github.com/megaease/easegress/v2/pkg/api"
	egLogger     = "github.com/megaease/easegress/v2/pkg/logger"
)

type ObjectConfig struct {
	Repo      string   `json:"repo"`
	Resources []string `json:"resources"`
	Filters   []string `json:"filters"`
}

// Const is a const definition.
type Const struct {
	Name    string
	Value   interface{}
	Comment string
}

// DefConst defines consts.
func DefConst(consts ...Const) *j.Statement {
	codes := []j.Code{}
	for i, c := range consts {
		if c.Comment != "" {
			codes = append(codes, j.Comment(c.Comment))
		}
		codes = append(codes, j.Id(c.Name).Op("=").Lit(c.Value))
		if i != len(consts)-1 {
			codes = append(codes, j.Line())
		}
	}
	return j.Const().Defs(codes...)
}

// Func is a function definition.
type Func struct {
	ReceiverName    string
	ReceiverType    string
	ReceiverPointer bool

	Name    string
	Params  []j.Code
	Returns []j.Code
	Block   []j.Code
}

// DefFunc defines a function.
func DefFunc(fn *Func) *j.Statement {
	res := j.Func()

	// If a receiver is provided, this is a method, not a function.
	if fn.ReceiverName != "" && fn.ReceiverType != "" {
		if fn.ReceiverPointer {
			res.Params(j.Id(fn.ReceiverName).Op("*").Id(fn.ReceiverType))
		} else {
			res.Params(j.Id(fn.ReceiverName).Id(fn.ReceiverType))
		}
	}

	if fn.Name != "" {
		res.Id(fn.Name)
	}

	res.Params(fn.Params...)
	if len(fn.Returns) > 0 {
		res.Add(fn.Returns...).Block(fn.Block...)
	} else {
		res.Block(fn.Block...)
	}
	return res
}

func WriteObjectConfigFile(dir string, config *ObjectConfig) error {
	fileName := path.Join(dir, objectConfigFileName)
	yamlData, err := codectool.MarshalYAML(config)
	if err != nil {
		return fmt.Errorf("write object config file failed, %s", err.Error())
	}
	return os.WriteFile(fileName, yamlData, os.ModePerm)
}

func ReadObjectConfigFile(dir string) (*ObjectConfig, error) {
	yamlData, err := os.ReadFile(path.Join(dir, objectConfigFileName))
	if err != nil {
		return nil, fmt.Errorf("read object config file %s failed, %s", objectConfigFileName, err.Error())
	}
	config := &ObjectConfig{}
	err = codectool.UnmarshalYAML(yamlData, config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal object config file %s failed, %s", objectConfigFileName, err.Error())
	}
	return config, nil
}
