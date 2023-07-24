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

package utils

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// GetFilterDir returns the filter directory.
func GetFilterDir(dir string) string {
	return path.Join(dir, "filters")
}

// GetResourceDir returns the resource directory.
func GetResourceDir(dir string) string {
	return path.Join(dir, "resources")
}

// GetFilterPath returns the filter path.
func GetFilterPath(dir, filter string) string {
	return path.Join(GetFilterDir(dir), strings.ToLower(filter))
}

// GetResourcePath returns the resource path.
func GetResourcePath(dir, resource string) string {
	return path.Join(GetResourceDir(dir), strings.ToLower(resource))
}

// GetFilterFileName returns the filter file name.
func GetFilterFileName(dir string, filter string) string {
	return path.Join(GetFilterPath(dir, filter), strings.ToLower(filter)+".go")
}

// GetResourceFileName returns the resource file name.
func GetResourceFileName(dir string, resource string) string {
	return path.Join(GetResourcePath(dir, resource), strings.ToLower(resource)+".go")
}

func GetRegistryDir(dir string) string {
	return path.Join(dir, "registry")
}

func GetRegistryFileName(dir string) string {
	return path.Join(GetRegistryDir(dir), "registry.go")
}

// MakeDirs makes the directories.
func MakeDirs(wd string, filters []string, resources []string) error {
	for _, f := range filters {
		err := os.MkdirAll(GetFilterPath(wd, f), os.ModePerm)
		if err != nil {
			return fmt.Errorf("make directory for filter %s failed: %s", f, err.Error())
		}
	}
	for _, r := range resources {
		err := os.MkdirAll(GetResourcePath(wd, r), os.ModePerm)
		if err != nil {
			return fmt.Errorf("make directory for resource %s failed: %s", r, err.Error())
		}
	}
	return nil
}
