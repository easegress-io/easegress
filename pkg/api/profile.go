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

package api

import (
	//"fmt"
	"net/http"

	//"github.com/megaease/easegress/pkg/option/option.go"
	"gopkg.in/yaml.v2"
)

const (
	// ProfilePrefix is the URL prefix of profile APIs
	ProfilePrefix = "/profile"
)

// ProfileStatusResponse contains cpu and memory profile file paths
type ProfileStatusResponse struct {
	CpuPath       string `yaml:"cpuPath"`
	MemoryPath    string `yaml:"memoryPath"`
}

func (s *Server) profileAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    ProfilePrefix,
			Method:  http.MethodGet,
			Handler: s.getProfileStatus,
		},
	}
}

func (s *Server) getProfileStatus(w http.ResponseWriter, r *http.Request) {
	cpuFile := s.opt.CPUProfileFile
	if cpuFile == "" {
		return
	}

	result := &ProfileStatusResponse{CpuPath: cpuFile}
	w.Header().Set("Content-Type", "text/vnd.yaml")
	err := yaml.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}
