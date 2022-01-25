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

package headerlookup

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of HeaderLookup.
	Kind = "HeaderLookup"
	// customDataPrefix is prefix for lookup data.
	customDataPrefix = "/custom-data/"
	// size of the LRU cache
	cacheSize = 128
)

var results = []string{}

func init() {
	httppipeline.Register(&HeaderLookup{})
}

type (
	// HeaderLookup retrieves values from etcd to headers.
	HeaderLookup struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
		etcdPrefix string
		headerKey  string
		pathRegExp *regexp.Regexp

		cache   *lru.Cache
		cluster cluster.Cluster
		stopCtx context.Context
		cancel  context.CancelFunc
	}

	// HeaderSetterSpec defines etcd source key and request destination header.
	HeaderSetterSpec struct {
		EtcdKey   string `yaml:"etcdKey,omitempty" jsonschema:"omitempty"`
		HeaderKey string `yaml:"headerKey,omitempty" jsonschema:"omitempty"`
	}

	// Spec defines header key and etcd prefix that form etcd key like /custom-data/{etcdPrefix}/{headerKey's value}.
	// This /custom-data/{etcdPrefix}/{headerKey's value} is retrieved from etcd and HeaderSetters extract keys from the
	// from the retrieved etcd item.
	// When PathRegExp is defined, PathRegExp is used with `regexp.FindStringSubmatch` to identify a group from path.
	// The first captured group is appended to the etcd key in following format:
	// /custom-data/{etcdPrefix}/{headerKey's value}-{regex group} . For example, for path
	// "/api/bananas/33" and pathRegExp: "^/api/([a-z]+)/[0-9]*", the group "bananas" is extracted and etcd key is
	// /custom-data/{etcdPrefix}/{headerKey's value}-bananas.
	Spec struct {
		HeaderKey     string              `yaml:"headerKey" jsonschema:"required"`
		EtcdPrefix    string              `yaml:"etcdPrefix" jsonschema:"required"`
		PathRegExp    string              `yaml:"pathRegExp" jsonschema:"omitempty"`
		HeaderSetters []*HeaderSetterSpec `yaml:"headerSetters" jsonschema:"required"`
	}
)

// Validate validates spec.
func (spec Spec) Validate() error {
	if spec.HeaderKey == "" {
		return fmt.Errorf("headerKey is required")
	}
	if spec.EtcdPrefix == "" {
		return fmt.Errorf("etcdPrefix is required")
	}
	if len(spec.HeaderSetters) < 1 {
		return fmt.Errorf("at least one headerSetter is required")
	}
	for _, hs := range spec.HeaderSetters {
		if hs.EtcdKey == "" {
			return fmt.Errorf("headerSetters[i].etcdKey is required")
		}
		if hs.HeaderKey == "" {
			return fmt.Errorf("headerSetters[i].headerKey is required")
		}
	}

	if _, err := regexp.Compile(spec.PathRegExp); err != nil {
		return err
	}
	return nil
}

// Kind returns the kind of HeaderLookup.
func (hl *HeaderLookup) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HeaderLookup.
func (hl *HeaderLookup) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of HeaderLookup.
func (hl *HeaderLookup) Description() string {
	return "HeaderLookup enriches request headers per request, looking up values from etcd."
}

// Results returns the results of HeaderLookup.
func (hl *HeaderLookup) Results() []string {
	return results
}

// Init initializes HeaderLookup.
func (hl *HeaderLookup) Init(filterSpec *httppipeline.FilterSpec) {
	hl.filterSpec, hl.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	if filterSpec.Super() != nil && filterSpec.Super().Cluster() != nil {
		hl.cluster = filterSpec.Super().Cluster()
	}
	hl.etcdPrefix = customDataPrefix + strings.TrimPrefix(hl.spec.EtcdPrefix, "/")
	hl.headerKey = http.CanonicalHeaderKey(hl.spec.HeaderKey)
	hl.cache, _ = lru.New(cacheSize)
	hl.stopCtx, hl.cancel = context.WithCancel(context.Background())
	hl.pathRegExp = regexp.MustCompile(hl.spec.PathRegExp)
	hl.watchChanges()
}

// Inherit inherits previous generation of HeaderLookup.
func (hl *HeaderLookup) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	hl.Init(filterSpec)
}

func (hl *HeaderLookup) lookup(headerVal string) (map[string]string, error) {
	if val, ok := hl.cache.Get(hl.etcdPrefix + headerVal); ok {
		return val.(map[string]string), nil
	}

	etcdVal, err := hl.cluster.Get(hl.etcdPrefix + headerVal)
	if err != nil {
		return nil, err
	}
	if etcdVal == nil {
		return nil, fmt.Errorf("no data found")
	}
	result := make(map[string]string, len(hl.spec.HeaderSetters))
	etcdValues := make(map[string]string)
	err = yaml.Unmarshal([]byte(*etcdVal), etcdValues)
	if err != nil {
		return nil, err
	}
	for _, setter := range hl.spec.HeaderSetters {
		if val, ok := etcdValues[setter.EtcdKey]; ok {
			result[setter.HeaderKey] = val
		}
	}

	hl.cache.Add(hl.etcdPrefix+headerVal, result)
	return result, nil
}

func findKeysToDelete(kvs map[string]string, cache *lru.Cache) []string {
	keysToDelete := []string{}
	intersection := make(map[string]string)
	for key, newValues := range kvs {
		if oldValues, ok := cache.Peek(key); ok {
			intersection[key] = ""
			if newValues != oldValues {
				keysToDelete = append(keysToDelete, key)
			}
		}
	}
	// delete cache items that were not in kvs
	for _, cacheKey := range cache.Keys() {
		if _, exists := intersection[cacheKey.(string)]; !exists {
			keysToDelete = append(keysToDelete, cacheKey.(string))
		}
	}
	return keysToDelete
}

func (hl *HeaderLookup) watchChanges() {
	var (
		syncer *cluster.Syncer
		err    error
		ch     <-chan map[string]string
	)

	for {
		syncer, err = hl.cluster.Syncer(30 * time.Minute)
		if err != nil {
			logger.Errorf("failed to create syncer: %v", err)
		} else if ch, err = syncer.SyncPrefix(hl.etcdPrefix); err != nil {
			logger.Errorf("failed to sync prefix: %v", err)
			syncer.Close()
		} else {
			break
		}

		select {
		case <-time.After(10 * time.Second):
		case <-hl.stopCtx.Done():
			return
		}
	}
	// start listening in background
	go func() {
		defer syncer.Close()

		for {
			select {
			case <-hl.stopCtx.Done():
				return
			case kvs := <-ch:
				logger.Infof("HeaderLookup update")
				keysToDelete := findKeysToDelete(kvs, hl.cache)
				for _, cacheKey := range keysToDelete {
					hl.cache.Remove(cacheKey)
				}
			}
		}
	}()
	return
}

// Close closes HeaderLookup.
func (hl *HeaderLookup) Close() {
	hl.cancel()
}

// Handle retrieves header values and sets request headers.
func (hl *HeaderLookup) Handle(ctx httpcontext.HTTPContext) string {
	result := hl.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (hl *HeaderLookup) handle(ctx httpcontext.HTTPContext) string {
	header := ctx.Request().Header()
	headerVal := header.Get(hl.headerKey)
	if headerVal == "" {
		logger.Warnf("request does not have header '%s'", hl.spec.HeaderKey)
		return ""
	}
	if hl.spec.PathRegExp != "" {
		path := ctx.Request().Path()
		if match := hl.pathRegExp.FindStringSubmatch(path); match != nil && len(match) > 1 {
			headerVal = headerVal + "-" + match[1]
		}
	}
	headersToAdd, err := hl.lookup(headerVal)
	if err != nil {
		logger.Errorf(err.Error())
		return ""
	}
	for hk, hv := range headersToAdd {
		header.Set(http.CanonicalHeaderKey(hk), hv)
	}
	return ""
}

// Status returns status.
func (hl *HeaderLookup) Status() interface{} { return nil }
