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

// Package api implements the HTTP API of Easegress.
package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func aboutText() string {
	return fmt.Sprintf(`Copyright © 2017 - %d The Easegress Authors. All rights reserved.
Powered by open-source software: Etcd(https://etcd.io), Apache License 2.0.
`, time.Now().Year())
}

const (
	// APIPrefixV1 is the prefix of v1 api, deprecated, will be removed soon.
	APIPrefixV1 = "/apis/v1"
	// APIPrefixV2 is the prefix of v2 api.
	APIPrefixV2 = "/apis/v2"

	lockKey = "/config/lock"

	// ConfigVersionKey is the key of header for config version.
	ConfigVersionKey = "X-Config-Version"
)

var (
	apisMutex      = sync.Mutex{}
	apis           = make(map[string]*Group)
	apisChangeChan = make(chan struct{}, 10)

	appendAddonAPIs []func(s *Server, group *Group)
)

type apisByOrder []*Group

func (a apisByOrder) Less(i, j int) bool { return a[i].Group < a[j].Group }
func (a apisByOrder) Len() int           { return len(a) }
func (a apisByOrder) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// RegisterAPIs registers global admin APIs.
func RegisterAPIs(apiGroup *Group) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	_, exists := apis[apiGroup.Group]
	if exists {
		logger.Errorf("group %s existed", apiGroup.Group)
	}
	apis[apiGroup.Group] = apiGroup

	logger.Infof("register api group %s", apiGroup.Group)
	apisChangeChan <- struct{}{}
}

// UnregisterAPIs unregisters the API group.
func UnregisterAPIs(group string) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	_, exists := apis[group]
	if !exists {
		logger.Errorf("group %s not found", group)
		return
	}

	delete(apis, group)

	logger.Infof("unregister api group %s", group)
	apisChangeChan <- struct{}{}
}

func (s *Server) registerAPIs() {
	group := &Group{
		Group: "admin",
	}
	group.Entries = append(group.Entries, s.listAPIEntries()...)
	group.Entries = append(group.Entries, s.memberAPIEntries()...)
	group.Entries = append(group.Entries, s.objectAPIEntries()...)
	group.Entries = append(group.Entries, s.metadataAPIEntries()...)
	group.Entries = append(group.Entries, s.healthAPIEntries()...)
	group.Entries = append(group.Entries, s.aboutAPIEntries()...)
	group.Entries = append(group.Entries, s.customDataAPIEntries()...)
	group.Entries = append(group.Entries, s.profileAPIEntries()...)
	group.Entries = append(group.Entries, s.prometheusMetricsAPIEntries()...)
	group.Entries = append(group.Entries, s.logsAPIEntries()...)

	for _, fn := range appendAddonAPIs {
		fn(s, group)
	}

	RegisterAPIs(group)
}

func (s *Server) listAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    "",
			Method:  "GET",
			Handler: s.listAPIs,
		},
	}
}

func (s *Server) healthAPIEntries() []*Entry {
	return []*Entry{
		{
			// https://stackoverflow.com/a/43381061/1705845
			Path:    "/healthz",
			Method:  "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) { /* 200 by default */ },
		},
	}
}

func (s *Server) aboutAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:   "/about",
			Method: "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(aboutText()))
			},
		},
	}
}

func (s *Server) listAPIs(w http.ResponseWriter, r *http.Request) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	apiGroups := make([]*Group, 0, len(apis))
	for _, group := range apis {
		apiGroups = append(apiGroups, group)
	}

	sort.Sort(apisByOrder(apiGroups))

	WriteBody(w, r, apiGroups)
}

// WriteBody writes the body to the response writer in proper format.
func WriteBody(w http.ResponseWriter, r *http.Request, body interface{}) {
	buff := codectool.MustMarshalJSON(body)
	contentType := "application/json"

	accpetHeader := r.Header.Get("Accept")
	if strings.Contains(accpetHeader, "yaml") {
		buff = codectool.MustJSONToYAML(buff)
		contentType = "text/x-yaml"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(buff)
}
