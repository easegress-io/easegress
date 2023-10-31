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

package nginx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type tempTestDir struct {
	dir   string
	t     *testing.T
	files []string
}

func newTempTestDir(t *testing.T) *tempTestDir {
	dir, err := os.MkdirTemp("", "test")
	require.Nil(t, err)
	return &tempTestDir{dir: dir, t: t}
}

func (dir *tempTestDir) Create(filename string, content []byte) string {
	file, err := os.Create(filepath.Join(dir.dir, filename))
	require.Nil(dir.t, err)
	defer file.Close()

	_, err = file.Write(content)
	require.Nil(dir.t, err)
	dir.files = append(dir.files, file.Name())
	return file.Name()
}

func (dir *tempTestDir) Clean() {
	for _, file := range dir.files {
		os.Remove(file)
	}
	os.Remove(dir.dir)
}

func printJson(t *testing.T, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	require.Nil(t, err)
	t.Log(string(b))
}
