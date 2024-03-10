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

// FilterInfo is the information of a filter.
type FilterInfo struct {
	ReceiverName string
	Name         string
	LowerName    string
	ResultFail   string
}

// NewFilterInfo creates a new Info.
func NewFilterInfo(name string) *FilterInfo {
	return &FilterInfo{
		ReceiverName: strings.ToLower(name[:1]),
		Name:         name,
		LowerName:    strings.ToLower(name),
		ResultFail:   fmt.Sprintf("result%sFail", name),
	}
}

func CreateFilter(name string) *j.File {
	info := NewFilterInfo(name)
	file := j.NewFile(info.LowerName)
	file.Comment("import " + egLogger).Line()
	file.ImportName(egFilters, "filters")
	file.ImportAlias(egContext, "egCtx")

	defineFilterVars(file, info)
	defineFilterStructs(file, info)
	defineFilterMethods(file, info)
	return file
}

func defineFilterVars(file *j.File, info *FilterInfo) {
	// define consts
	consts := Consts{
		{
			Name:    info.ResultFail,
			Value:   fmt.Sprintf("%sFail", info.Name),
			Comment: fmt.Sprintf("%s is the result when %s.Handle(ctx) fails. You can add more results here, please remember to update %s.Results", info.ResultFail, info.Name, KindName),
		},
	}
	file.Add(consts.Def())

	// define filters.Kind
	file.Comment("Kind is the kind of the %s filter. It is used to register and create a filter instance.")
	file.Var().Id(KindName).Op("=").Op("&").Qual(egFilters, "Kind").Values(j.Dict{
		j.Id("Name"):        j.Lit(info.Name),
		j.Id("Description"): j.Lit(fmt.Sprintf("%s is a custom filter", info.Name)),
		j.Id("Results"):     j.Index().String().Values(j.Id(info.ResultFail)),
		j.Id("DefaultSpec"): (&Func{
			Returns: []j.Code{j.Qual(egFilters, "Spec")},
			Block:   []j.Code{j.Return(j.Op("&").Id(SpecName).Values(j.Dict{}))},
		}).Def(),
		j.Id("CreateInstance"): (&Func{
			Params:  []j.Code{j.Id("spec").Qual(egFilters, "Spec")},
			Returns: []j.Code{j.Qual(egFilters, "Filter")},
			Block:   []j.Code{j.Return(j.Op("&").Id(info.Name).Values(j.Dict{j.Id("spec"): j.Id("spec").Dot("(*Spec)")}))},
		}).Def(),
	})
}

func defineFilterStructs(file *j.File, info *FilterInfo) {
	file.Comment(fmt.Sprintf("%s is a custom filter", info.Name))
	file.Type().Id(info.Name).Struct(
		j.Id("spec").Op("*").Id("Spec"),
		j.Line(),
		j.Comment(yourCodeHere),
	)

	file.Comment(fmt.Sprintf("%s is the spec of %s", SpecName, info.Name))
	file.Type().Id(SpecName).Struct(
		j.Qual(egFilters, "BaseSpec").Tag(map[string]string{"json": ",inline"}),
		j.Line(),
		j.Comment(yourCodeHere),
	)

	file.Var().Op("_").Qual(egFilters, "Filter").Op("=").Parens(j.Op("*").Id(info.Name)).Parens(j.Nil())
	file.Var().Op("_").Qual(egFilters, "Spec").Op("=").Parens(j.Op("*").Id(SpecName)).Parens(j.Nil())
}

func defineFilterMethods(file *j.File, info *FilterInfo) {
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

	// define filter Name method
	file.Comment("Name returns the name of the filter.")
	nameFunc := receiver()
	nameFunc.Name = "Name"
	nameFunc.Returns = []j.Code{j.String()}
	nameFunc.Block = []j.Code{j.Return(j.Id(info.ReceiverName).Dot("spec").Dot("Name()"))}
	file.Add(nameFunc.Def())

	// define filter Kind method
	file.Comment("Kind returns the kind of the filter.")
	kindFunc := receiver()
	kindFunc.Name = "Kind"
	kindFunc.Returns = []j.Code{j.Op("*").Qual(egFilters, "Kind")}
	kindFunc.Block = []j.Code{j.Return(j.Id(KindName))}
	file.Add(kindFunc.Def())

	// define filter Spec method
	file.Comment("Spec returns the spec of the filter.")
	specFunc := receiver()
	specFunc.Name = "Spec"
	specFunc.Returns = []j.Code{j.Qual(egFilters, "Spec")}
	specFunc.Block = []j.Code{j.Return(j.Id(info.ReceiverName).Dot("spec"))}
	file.Add(specFunc.Def())

	// define filter Init method
	file.Comment("Init initializes the filter.")
	initFunc := receiver()
	initFunc.Name = "Init"
	initFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
	}
	file.Add(initFunc.Def())

	// define filter Inherit method
	file.Comment("Inherit inherits previous filter.")
	inheritFunc := receiver()
	inheritFunc.Name = "Inherit"
	inheritFunc.Params = []j.Code{j.Id("previousGeneration").Qual(egFilters, "Filter")}
	inheritFunc.Block = []j.Code{
		j.Comment("Pipeline will close the previous filter automatically."),
		j.Comment(yourCodeHere),
	}
	file.Add(inheritFunc.Def())

	// define filter Handle method
	file.Comment("Handle handles the request.")
	handleFunc := receiver()
	handleFunc.Name = "Handle"
	handleFunc.Params = []j.Code{j.Id("ctx").Op("*").Qual(egContext, "Context")}
	handleFunc.Returns = []j.Code{j.String()}
	handleFunc.Block = []j.Code{
		j.Comment("hint: req := ctx.GetRequest().(*httpprot.Request)"),
		j.Comment("empty string means success"),
		j.Comment(yourCodeHere),
		j.Return(j.Lit("")),
	}
	file.Add(handleFunc.Def())

	// define filter Status method
	file.Comment("Status returns the status of the filter.")
	statusFunc := receiver()
	statusFunc.Name = "Status"
	statusFunc.Returns = []j.Code{j.Interface()}
	statusFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
		j.Return(j.Nil()),
	}
	file.Add(statusFunc.Def())

	// define filter Close method
	file.Comment("Close closes the filter.")
	closeFunc := receiver()
	closeFunc.Name = "Close"
	closeFunc.Block = []j.Code{
		j.Comment(yourCodeHere),
	}
	file.Add(closeFunc.Def())
}
