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

package function

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	cron "github.com/robfig/cron/v3"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/codecounter"
)

type (
	// Cron is the cron job for http probe.
	Cron struct {
		mu sync.Mutex

		url     string
		cronJob *cron.Cron

		total   uint64
		succeed uint64
		failed  uint64
		cc      *codecounter.CodeCounter
	}

	// CronSpec is the spec of Cron.
	CronSpec struct {
		WithSecond bool   `yaml:"withSecond"`
		Spec       string `yaml:"spec" jsonschema:"required"`

		// TODO?: Add request adaptor stuff to customize request.
	}

	// CronStatus is the status of Cron.
	CronStatus struct {
		Total   uint64
		Succeed uint64
		Failed  uint64
		Codes   map[int]uint64
	}
)

// Validate validates CronSpec.
func (spec CronSpec) Validate() error {
	_, err := cron.NewParser(spec.parseOpt()).Parse(spec.Spec)
	if err != nil {
		return fmt.Errorf("parse cron spec %s failed: %v",
			spec.Spec, err)
	}

	return nil
}

func (spec CronSpec) parseOpt() cron.ParseOption {
	opt := withoutSecondOpt
	if spec.WithSecond {
		opt = withSecondOpt
	}

	return opt
}

// NewCron creates a Cron.
func NewCron(url string, spec *CronSpec) *Cron {
	c := &Cron{
		url: url,

		cc: codecounter.New(),
	}

	cronJob := cron.New(cron.WithParser(cron.NewParser(spec.parseOpt())))
	_, err := cronJob.AddFunc(spec.Spec, c.run)
	if err != nil {
		logger.Errorf("BUG: add cron job %s failed: %v", spec.Spec, err)
		return nil
	}

	c.cronJob = cronJob
	c.cronJob.Start()

	return c
}

func (c *Cron) run() {
	resp, err := http.Get(c.url)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.total++
		c.failed++
		return
	}
	go func() {
		// NOTE: Need to be read to completion and closed.
		// Reference: https://golang.org/pkg/net/http/#Response
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
	}()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.total++
	c.succeed++
	c.cc.Count(resp.StatusCode)
}

// Status returns the status of Cron.
func (c *Cron) Status() *CronStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	return &CronStatus{
		Total:   c.total,
		Succeed: c.succeed,
		Failed:  c.failed,
		Codes:   c.cc.Codes(),
	}
}

// Close closes the CronJob.
func (c *Cron) Close() {
	c.cronJob.Stop()
}
