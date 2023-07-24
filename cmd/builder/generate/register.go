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

// Package generate generates codes for egbuilder.
package generate

import j "github.com/dave/jennifer/jen"

type Objects struct {
	Resources []string `json:"resources"`
	Filters   []string `json:"filters"`
	Repo      []string `json:"repo"`
}

func CreateRegister(objects *Objects) *j.File {
	file := j.NewFile("register")
	file.Comment("import " + egLogger).Line()
	file.ImportName(egFilters, "filters")

	defineRegisterInit(file, objects)
	return file
}

func defineRegisterInit(file *j.File, objects *Objects) {
}
