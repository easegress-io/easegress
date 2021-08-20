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

package retryer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

const (
	// Kind is the kind of Retryer.
	Kind = "Retryer"
)

var results = []string{}

func init() {
	httppipeline.Register(&Retryer{})
}

type (
	backOffPolicy uint8

	// Policy is the policy of the retryer
	Policy struct {
		Name                 string `yaml:"name" jsonschema:"required"`
		MaxAttempts          int    `yaml:"maxAttempts" jsonschema:"omitempty,minimum=1"`
		WaitDuration         string `yaml:"waitDuration" jsonschema:"omitempty,format=duration"`
		waitDuration         time.Duration
		BackOffPolicy        string  `yaml:"backOffPolicy" jsonschema:"omitempty,enum=random,enum=exponential"`
		RandomizationFactor  float64 `yaml:"randomizationFactor" jsonschema:"omitempty,minimum=0,maximum=1"`
		backOffPolicy        backOffPolicy
		CountingNetworkError bool  `yaml:"countingNetworkError" jsonschema:"omitempty"`
		FailureStatusCodes   []int `yaml:"failureStatusCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
	}

	// URLRule is the URL rule
	URLRule struct {
		urlrule.URLRule `yaml:",inline"`
		policy          *Policy
	}

	// Spec is the spec of retryer
	Spec struct {
		Policies         []*Policy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string     `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	// Retryer is the struct of retryer
	Retryer struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
	}
)

const (
	randomBackOff backOffPolicy = iota
	exponentiallyBackOff
)

// Validate implements custom validation for Spec
func (spec Spec) Validate() error {
URLLoop:
	for _, u := range spec.URLs {
		name := u.PolicyRef
		if name == "" {
			name = spec.DefaultPolicyRef
		}

		for _, p := range spec.Policies {
			if p.Name == name {
				continue URLLoop
			}
		}

		return fmt.Errorf("policy '%s' is not defined", name)
	}

	return nil
}

// Kind returns the kind of Retryer.
func (r *Retryer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Retryer.
func (r *Retryer) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Retryer
func (r *Retryer) Description() string {
	return "Retryer implements a retryer for http request."
}

// Results returns the results of Retryer.
func (r *Retryer) Results() []string {
	return results
}

func (r *Retryer) initURL(u *URLRule) {
	u.Init()

	name := u.PolicyRef
	if name == "" {
		name = r.spec.DefaultPolicyRef
	}

	for _, p := range r.spec.Policies {
		if p.Name == name {
			u.policy = p
			break
		}
	}

	if u.policy.MaxAttempts <= 0 {
		u.policy.MaxAttempts = 3
	}

	if u.policy.RandomizationFactor < 0 || u.policy.RandomizationFactor >= 1 {
		u.policy.RandomizationFactor = 0
	}

	if strings.ToUpper(u.policy.BackOffPolicy) == "EXPONENTIAL" {
		u.policy.backOffPolicy = exponentiallyBackOff
	}

	if d := u.policy.WaitDuration; d != "" {
		u.policy.waitDuration, _ = time.ParseDuration(d)
	} else {
		u.policy.waitDuration = time.Millisecond * 500
	}
}

// Init initializes Retryer.
func (r *Retryer) Init(filterSpec *httppipeline.FilterSpec) {
	r.filterSpec = filterSpec
	r.spec = filterSpec.FilterSpec().(*Spec)
	for _, url := range r.spec.URLs {
		r.initURL(url)
	}
}

// Inherit inherits previous generation of Retryer.
func (r *Retryer) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	r.Init(filterSpec)
}

func (r *Retryer) handle(ctx context.HTTPContext, u *URLRule) string {
	attempt := 0
	base := float64(u.policy.waitDuration)

	data, _ := ioutil.ReadAll(ctx.Request().Body())
	for {
		attempt++
		ctx.Request().SetBody(bytes.NewReader(data))

		result := ctx.CallNextHandler("")

		statusCode := ctx.Response().StatusCode()
		hasErr := u.policy.CountingNetworkError && context.IsNetworkError(statusCode)
		if !hasErr {
			for _, c := range u.policy.FailureStatusCodes {
				if statusCode == c {
					hasErr = true
					break
				}
			}
		}

		if !hasErr {
			ctx.AddTag(fmt.Sprintf("retryer: succeeded after %d attempts", attempt))
			ctx.Response().Std().Header().Set("X-Mesh-Retryer", fmt.Sprintf("Succeeded-after-%d-attempts", attempt))
			return result
		}

		logger.Infof("attempts %d of retryer %s on URL(%s) failed at %d, result is '%s'",
			attempt,
			r.filterSpec.Name(),
			u.ID(),
			time.Now().UnixNano()/1e6,
			result,
		)

		if attempt == u.policy.MaxAttempts {
			ctx.AddTag(fmt.Sprintf("retryer: failed after %d attempts", attempt))
			ctx.Response().Std().Header().Set("X-EG-Retryer", fmt.Sprintf("Failed-after-%d-attempts", attempt))
			return result
		}

		delta := base * u.policy.RandomizationFactor
		d := base - delta + float64(rand.Intn(int(delta*2+1)))
		timer := time.NewTimer(time.Duration(d))

		select {
		case <-ctx.Done():
			timer.Stop()
			return result
		case <-timer.C:
		}

		if u.policy.backOffPolicy == exponentiallyBackOff {
			base *= 1.5
		}
	}
}

// Handle handles HTTP request
func (r *Retryer) Handle(ctx context.HTTPContext) string {
	for _, u := range r.spec.URLs {
		if u.Match(ctx.Request()) {
			return r.handle(ctx, u)
		}
	}
	return ctx.CallNextHandler("")
}

// Status returns Status generated by Runtime.
func (r *Retryer) Status() interface{} {
	return nil
}

// Close closes Retryer.
func (r *Retryer) Close() {
}
