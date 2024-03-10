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

package gen

import (
	"fmt"
	"strings"

	j "github.com/dave/jennifer/jen"
)

// ControllerInfo is the information of a controller.
// Name: "TestFilter", used for struct name
// LowerName: "testfilter", used for file name, pkg name
// VarKind: "TestFilterKind", used for registry
// ResultFail: "resultTestFilterFail", used for result
// Most of these fields are used as identifier.
type ControllerInfo struct {
	ReceiverName string
	Name         string
	LowerName    string
}

// NewInfo creates a new Info.
func NewControllerInfo(name string) *ControllerInfo {
	return &ControllerInfo{
		ReceiverName: strings.ToLower(name[:1]),
		Name:         name,
		LowerName:    strings.ToLower(name),
	}
}

func CreateController(name string) *j.File {
	info := NewControllerInfo(name)
	file := j.NewFile(info.LowerName)
	file.Comment("import " + egLogger).Line()
	file.ImportName(egSupervisor, "supervisor")
	file.ImportName(egAPI, "api")
	file.ImportAlias(egContext, "egCtx")

	defineControllerVars(file, info)
	defineControllerStructs(file, info)
	defineControllerMethods(file, info)
	return file
}

func defineControllerVars(file *j.File, info *ControllerInfo) {
	file.Comment("API defines api controllers used by egctl")
	file.Var().Id("API").Op("=").Op("&").Qual(egAPI, "APIResource").Values(j.Dict{
		j.Id("Kind"):    j.Lit(info.Name),
		j.Id("Name"):    j.Lit(info.LowerName),
		j.Id("Aliases"): j.Index().String().Values(j.Lit(info.LowerName + "s")),
	})
}

func defineControllerStructs(file *j.File, info *ControllerInfo) {
	file.Comment(fmt.Sprintf("%s is a custom controller", info.Name))
	file.Type().Id(info.Name).Struct(
		j.Id("spec").Op("*").Id("Spec"),
		j.Line(),
		j.Comment(yourCodeHere),
	)

	file.Comment(fmt.Sprintf("%s is the spec of %s", SpecName, info.Name))
	file.Type().Id(SpecName).Struct(
		j.Comment(yourCodeHere),
	)

	file.Var().Op("_").Qual(egSupervisor, "TrafficObject").Op("=").Parens(j.Op("*").Id(info.Name)).Parens(j.Nil())
}

func defineControllerMethods(file *j.File, info *ControllerInfo) {
	// define spec Validate method
	file.Comment(fmt.Sprintf("Validate validates %s", SpecName))
	file.Add((&Func{
		ReceiverName:    "s",
		ReceiverType:    SpecName,
		ReceiverPointer: true,
		Name:            "Validate",
		Returns:         *j.Error(),
		Block: []j.Code{
			j.Comment(yourCodeHere),
			j.Return(j.Nil()),
		},
	}).Def())

	receiver := func() *Func {
		return &Func{
			ReceiverName:    info.ReceiverName,
			ReceiverType:    info.Name,
			ReceiverPointer: true,
		}
	}

	// define controller Category method
	file.Comment("Category returns the category of controller.")
	categoryFunc := receiver()
	categoryFunc.Name = "Category"
	categoryFunc.Returns = []j.Code{j.Qual(egSupervisor, "ObjectCategory")}
	categoryFunc.Block = []j.Code{
		j.Return(j.Qual(egSupervisor, "CategoryTrafficGate")),
	}
	file.Add(categoryFunc.Def())

	// define controller Kind method
	file.Comment("Kind returns the kind name of controller.")
	kindFunc := receiver()
	kindFunc.Name = "Kind"
	kindFunc.Returns = []j.Code{j.String()}
	kindFunc.Block = []j.Code{
		j.Return(j.Lit(info.Name)),
	}
	file.Add(kindFunc.Def())

	// define controller DefaultSpec method
	file.Comment("DefaultSpec returns the default spec of controller. It is used to unmarshal yaml file.")
	defaultSpecFunc := receiver()
	defaultSpecFunc.Name = "DefaultSpec"
	defaultSpecFunc.Returns = []j.Code{j.Interface()}
	defaultSpecFunc.Block = []j.Code{
		j.Return(j.Op("&").Id(SpecName).Values(j.Dict{})),
	}
	file.Add(defaultSpecFunc.Def())

	// define controller Status method
	file.Comment("Status returns the status of controller.")
	statusFunc := receiver()
	statusFunc.Name = "Status"
	statusFunc.Returns = []j.Code{j.Op("*").Qual(egSupervisor, "Status")}
	statusFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
		j.Return(j.Op("&").Qual(egSupervisor, "Status").Values(j.Dict{})),
	}
	file.Add(statusFunc.Def())

	// define controller Close method
	file.Comment("Close closes the controller.")
	closeFunc := receiver()
	closeFunc.Name = "Close"
	closeFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
	}
	file.Add(closeFunc.Def())

	// define controller Init method.
	file.Comment("Init initializes the controller.")
	file.Comment("muxMapper is used to get Pipeline by name, and can be used to handle request.")
	file.Comment("If you don't need to handle request, you can ignore it.")
	initFunc := receiver()
	initFunc.Name = "Init"
	initFunc.Params = []j.Code{
		j.Id("superSpec").Op("*").Qual(egSupervisor, "Spec"),
		j.Id("muxMapper").Qual(egContext, "MuxMapper"),
	}
	initFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
		j.Comment("spec := superSpec.ObjectSpec().(*Spec)"),
	}
	file.Add(initFunc.Def())

	// define controller Inherit method.
	file.Comment("Inherit inherits the controller from previous generation.")
	file.Comment("It is used when update the yaml file of controller.")
	inheritFunc := receiver()
	inheritFunc.Name = "Inherit"
	inheritFunc.Params = []j.Code{
		j.Id("superSpec").Op("*").Qual(egSupervisor, "Spec"),
		j.Id("previousGeneration").Qual(egSupervisor, "Object"),
		j.Id("muxMapper").Qual(egContext, "MuxMapper"),
	}
	inheritFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
		j.Comment("previousGeneration.Close()"),
		j.Comment(fmt.Sprintf("%s.Init(superSpec, muxMapper)", info.ReceiverName)),
	}
	file.Add(inheritFunc.Def())
}
