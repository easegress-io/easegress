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
	"path"

	j "github.com/dave/jennifer/jen"
)

func CreateServer(plugins []string) *j.File {
	file := j.NewFile("main")
	file.ImportName(egCmd, "cmd")
	file.Anon(egRegistry)

	for _, plugin := range plugins {
		file.Anon(path.Join(plugin, moduleRegistry))
	}

	mainFunc := &Func{
		Name: "main",
		Block: []j.Code{
			j.Qual(egCmd, "RunServer").Call(),
		},
	}
	file.Add(mainFunc.Def())
	return file
}
