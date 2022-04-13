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
	"fmt"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

const (
	// Kind is the kind of Retryer.
	Kind = "Retryer"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Retryer implements a retryer for http request.",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Retryer{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
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
		filters.BaseSpec `yaml:",inline"`

		Policies         []*Policy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string     `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	// Retryer is the struct of retryer
	Retryer struct {
		filterSpec *filters.Spec
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

// Name returns the name of the Retryer filter instance.
func (r *Retryer) Name() string {
	return r.spec.Name()
}

// Kind returns the kind of Retryer.
func (r *Retryer) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Retryer
func (r *Retryer) Spec() filters.Spec {
	return r.spec
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
func (r *Retryer) Init() {
	for _, url := range r.spec.URLs {
		r.initURL(url)
	}
}

// Inherit inherits previous generation of Retryer.
func (r *Retryer) Inherit(previousGeneration filters.Filter) {
	r.Init()
}

func (r *Retryer) handle(ctx *context.Context, u *URLRule) string {
	// TODO
	/*
		attempt := 0
		base := float64(u.policy.waitDuration)

		data, _ := io.ReadAll(ctx.Request().Body())
		for {
			attempt++
			ctx.Request().SetBody(bytes.NewReader(data), true)

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
				r.spec.Name(),
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
	*/
	return ""
}

// Handle handles HTTP request
func (r *Retryer) Handle(ctx *context.Context) string {
	// TODO:
	/*
		for _, u := range r.spec.URLs {
			if u.Match(ctx.Request()) {
				return r.handle(ctx, u)
			}
		}
		return ctx.CallNextHandler("")
	*/
	return ""
}

// Status returns Status generated by Runtime.
func (r *Retryer) Status() interface{} {
	return nil
}

// Close closes Retryer.
func (r *Retryer) Close() {
}
