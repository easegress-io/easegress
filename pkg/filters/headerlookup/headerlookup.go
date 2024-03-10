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

// Package headerlookup implements a filter that enriches request headers
// per request, looking up values from etcd.
package headerlookup

import (
	stdcontext "context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Kind is the kind of HeaderLookup.
	Kind = "HeaderLookup"
	// customDataPrefix is prefix for lookup data.
	customDataPrefix = "/custom-data/"
	// size of the LRU cache
	cacheSize = 128
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "HeaderLookup enriches request headers per request, looking up values from etcd.",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HeaderLookup{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// HeaderLookup retrieves values from etcd to headers.
	HeaderLookup struct {
		spec       *Spec
		etcdPrefix string
		headerKey  string
		pathRegExp *regexp.Regexp

		cache   *lru.Cache
		cluster cluster.Cluster
		stopCtx stdcontext.Context
		cancel  stdcontext.CancelFunc
	}

	// HeaderSetterSpec defines etcd source key and request destination header.
	HeaderSetterSpec struct {
		EtcdKey   string `json:"etcdKey,omitempty"`
		HeaderKey string `json:"headerKey,omitempty"`
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
		filters.BaseSpec `json:",inline"`

		HeaderKey     string              `json:"headerKey" jsonschema:"required"`
		EtcdPrefix    string              `json:"etcdPrefix" jsonschema:"required"`
		PathRegExp    string              `json:"pathRegExp,omitempty"`
		HeaderSetters []*HeaderSetterSpec `json:"headerSetters" jsonschema:"required"`
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

// Name returns the name of the HeaderLookup filter instance.
func (hl *HeaderLookup) Name() string {
	return hl.spec.Name()
}

// Kind returns the kind of HeaderLookup.
func (hl *HeaderLookup) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the HeaderLookup
func (hl *HeaderLookup) Spec() filters.Spec {
	return hl.spec
}

// Init initializes HeaderLookup.
func (hl *HeaderLookup) Init() {
	spec := hl.spec
	if spec.Super() != nil && spec.Super().Cluster() != nil {
		hl.cluster = spec.Super().Cluster()
	}
	hl.etcdPrefix = customDataPrefix + strings.TrimPrefix(hl.spec.EtcdPrefix, "/")
	hl.headerKey = http.CanonicalHeaderKey(hl.spec.HeaderKey)
	hl.cache, _ = lru.New(cacheSize)
	hl.stopCtx, hl.cancel = stdcontext.WithCancel(stdcontext.Background())
	hl.pathRegExp = regexp.MustCompile(hl.spec.PathRegExp)
	hl.watchChanges()
}

// Inherit inherits previous generation of HeaderLookup.
func (hl *HeaderLookup) Inherit(previousGeneration filters.Filter) {
	hl.Init()
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
		return nil, fmt.Errorf("no data for key %s found", hl.etcdPrefix+headerVal)
	}
	result := make(map[string]string, len(hl.spec.HeaderSetters))
	etcdValues := make(map[string]string)
	err = codectool.Unmarshal([]byte(*etcdVal), &etcdValues)
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
		syncer cluster.Syncer
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
}

// Close closes HeaderLookup.
func (hl *HeaderLookup) Close() {
	hl.cancel()
}

// Handle retrieves header values and sets request headers.
func (hl *HeaderLookup) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	header := req.HTTPHeader()
	headerVal := header.Get(hl.headerKey)
	if headerVal == "" {
		logger.Warnf("request does not have header '%s'", hl.spec.HeaderKey)
		return ""
	}
	// TODO: now headerlookup need path which make it only support for http protocol!
	// this may need update later
	if hl.spec.PathRegExp != "" {
		path := req.Path()
		if match := hl.pathRegExp.FindStringSubmatch(path); len(match) > 1 {
			headerVal = headerVal + "-" + match[1]
		}
	}
	headersToAdd, err := hl.lookup(headerVal)
	if err != nil {
		logger.Errorf(err.Error())
		return ""
	}
	for hk, hv := range headersToAdd {
		header.Set(hk, hv)
	}
	return ""
}

// Status returns status.
func (hl *HeaderLookup) Status() interface{} { return nil }
