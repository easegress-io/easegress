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
	"path/filepath"
	"strings"
)

const (
	egFilters    = "github.com/megaease/easegress/v2/pkg/filters"
	egContext    = "github.com/megaease/easegress/v2/pkg/context"
	egSupervisor = "github.com/megaease/easegress/v2/pkg/supervisor"
	egAPI        = "github.com/megaease/easegress/v2/pkg/api"
	egLogger     = "github.com/megaease/easegress/v2/pkg/logger"
	egCmd        = "github.com/megaease/easegress/v2/cmd"
	egRegistry   = "github.com/megaease/easegress/v2/pkg/registry"

	moduleFilter     = "filters"
	moduleController = "controllers"
	moduleRegistry   = "registry"
)

func getModulePath(dir string, moduleType string, moduleName string, importPath bool) string {
	moduleName = strings.ToLower(moduleName)
	join := func(paths ...string) string {
		if importPath {
			return path.Join(paths...)
		}
		return filepath.Join(paths...)
	}
	if moduleName != "" {
		return join(dir, moduleType, moduleName)
	}
	return join(dir, moduleType)
}

func getFileName(dir string, moduleType string, moduleName string, fileName string) string {
	moduleName = strings.ToLower(moduleName)
	fileName = strings.ToLower(fileName)
	if !strings.HasSuffix(fileName, ".go") {
		fileName += ".go"
	}
	if moduleName != "" {
		return filepath.Join(dir, moduleType, moduleName, fileName)
	}
	return filepath.Join(dir, moduleType, fileName)
}
