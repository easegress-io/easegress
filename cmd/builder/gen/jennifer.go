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
	j "github.com/dave/jennifer/jen"
)

type Consts []*Const

// Const is a const definition.
type Const struct {
	Name    string
	Value   interface{}
	Comment string
}

func (consts Consts) Def() *j.Statement {
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

func (fn *Func) Def() *j.Statement {
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
