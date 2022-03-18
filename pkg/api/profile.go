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
	"fmt"
	"gopkg.in/yaml.v2"
	"net/http"
)

const (
	// ProfilePrefix is the URL prefix of profile APIs
	ProfilePrefix = "/profile"
	// StartAction is the URL for starting profiling
	StartAction = "start"
	// StopAction is the URL for stopping profiling
	StopAction = "stop"
)

type (
	// ProfileStatusResponse contains cpu and memory profile file paths
	ProfileStatusResponse struct {
		CPUPath    string `yaml:"cpuPath"`
		MemoryPath string `yaml:"memoryPath"`
	}

	// StartProfilingRequest contains file path to profile file
	StartProfilingRequest struct {
		Path string `yaml:"path"`
	}
)

func (s *Server) profileAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    ProfilePrefix,
			Method:  http.MethodGet,
			Handler: s.getProfileStatus,
		},
		{
			Path:    fmt.Sprintf("%s/%s/cpu", ProfilePrefix, StartAction),
			Method:  http.MethodPost,
			Handler: s.startCPUProfile,
		},
		{
			Path:    fmt.Sprintf("%s/%s/memory", ProfilePrefix, StartAction),
			Method:  http.MethodPost,
			Handler: s.startMemoryProfile,
		},
		{
			Path:    fmt.Sprintf("%s/%s", ProfilePrefix, StopAction),
			Method:  http.MethodPost,
			Handler: s.stopProfile,
		},
	}
}

func (s *Server) getProfileStatus(w http.ResponseWriter, r *http.Request) {
	cpuFile := s.profile.CPUFileName()
	memFile := s.profile.MemoryFileName()

	result := &ProfileStatusResponse{CPUPath: cpuFile, MemoryPath: memFile}
	w.Header().Set("Content-Type", "text/vnd.yaml")
	err := yaml.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (s *Server) startCPUProfile(w http.ResponseWriter, r *http.Request) {
	spr := StartProfilingRequest{}
	err := yaml.NewDecoder(r.Body).Decode(&spr)
	if err != nil {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("bad request"))
		return
	}

	if spr.Path == "" {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("missing path"))
		return
	}
	s.profile.UpdateCPUProfile(spr.Path)
}

func (s *Server) startMemoryProfile(w http.ResponseWriter, r *http.Request) {
	spr := StartProfilingRequest{}
	err := yaml.NewDecoder(r.Body).Decode(&spr)
	if err != nil {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("bad request"))
		return
	}

	if spr.Path == "" {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("missing path"))
		return
	}
	// Memory profile is flushed only at stop/exit
	s.profile.UpdateMemoryProfile(spr.Path)
}

func (s *Server) stopProfile(w http.ResponseWriter, r *http.Request) {
	s.profile.StopCPUProfile()
	s.profile.MemoryProfile(s.profile.MemoryFileName())
}
