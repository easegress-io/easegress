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

// Package profile provides profile related functions.
package profile

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
)

// Profile is the Profile interface.
type Profile interface {
	StartCPUProfile(fp string) error

	UpdateCPUProfile(fp string) error
	UpdateMemoryProfile(fp string)

	StopCPUProfile()
	StopMemoryProfile(fp string)

	CPUFileName() string
	MemoryFileName() string

	Close(wg *sync.WaitGroup)
	Lock()
	Unlock()
}

type profile struct {
	opt         *option.Options
	cpuFile     *os.File
	cpuFileName string
	memFileName string

	mutex sync.Mutex
}

// New creates a profile.
func New(opt *option.Options) (Profile, error) {
	p := &profile{
		opt: opt,
	}

	err := p.StartCPUProfile("")
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *profile) CPUFileName() string {
	if p.cpuFile == nil {
		return p.cpuFileName
	}
	return p.cpuFile.Name()
}

func (p *profile) MemoryFileName() string {
	return p.memFileName
}

func (p *profile) UpdateMemoryProfile(fp string) {
	p.memFileName = fp
}

func (p *profile) StartCPUProfile(filepath string) error {
	if p.opt.CPUProfileFile == "" && filepath == "" {
		return nil
	}
	if filepath == "" {
		filepath = p.opt.CPUProfileFile
	}

	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create cpu profile failed: %v", err)
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		return fmt.Errorf("start cpu profile failed: %v", err)
	}

	p.cpuFile = f
	p.cpuFileName = f.Name()

	logger.Infof("cpu profile: %s", filepath)

	return nil
}

func (p *profile) StopMemoryProfile(filepath string) {
	if p.opt.MemoryProfileFile == "" && filepath == "" {
		return
	}
	if filepath == "" {
		filepath = p.opt.MemoryProfileFile
	}

	// to include every allocated block in the profile
	runtime.MemProfileRate = 1

	logger.Infof("memory profile: %s", filepath)
	f, err := os.Create(filepath)
	if err != nil {
		logger.Errorf("create memory profile failed: %v", err)
		return
	}

	runtime.GC()         // get up-to-date statistics
	debug.FreeOSMemory() // help developer when using outside monitor tool

	if err := pprof.WriteHeapProfile(f); err != nil {
		logger.Errorf("write memory file failed: %v", err)
		return
	}
	if err := f.Close(); err != nil {
		logger.Errorf("close memory file failed: %v", err)
		return
	}
}

func (p *profile) StopCPUProfile() {
	if p.cpuFile != nil {
		fname := p.cpuFile.Name()
		pprof.StopCPUProfile()
		err := p.cpuFile.Close()
		if err != nil {
			logger.Errorf("close %s failed: %v", fname, err)
		}
		p.cpuFile = nil
	}
}

func (p *profile) UpdateCPUProfile(filepath string) error {
	p.StopCPUProfile()
	return p.StartCPUProfile(filepath)
}

func (p *profile) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	p.StopCPUProfile()
	p.StopMemoryProfile("")
}

func (p *profile) Lock() {
	p.mutex.Lock()
}

func (p *profile) Unlock() {
	p.mutex.Unlock()
}
