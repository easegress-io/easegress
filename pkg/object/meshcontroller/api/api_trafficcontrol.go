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

package api

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easemesh-api/v2alpha1"
)

func (a *API) listHTTPRouteGroups(w http.ResponseWriter, r *http.Request) {
	groups := a.service.ListHTTPRouteGroups()
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	pbGroups := make([]*v2alpha1.HTTPRouteGroup, 0, len(groups))
	for _, v := range groups {
		group := &v2alpha1.HTTPRouteGroup{}
		err := a.convertSpecToPB(v, group)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbGroups = append(pbGroups, group)
	}

	buff := codectool.MustMarshalJSON(pbGroups)
	a.writeJSONBody(w, buff)
}

func (a *API) getHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	group := a.service.GetHTTPRouteGroup(name)
	if group == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbGroup := &v2alpha1.HTTPRouteGroup{}
	err = a.convertSpecToPB(group, pbGroup)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", group, err))
	}

	buff := codectool.MustMarshalJSON(pbGroup)
	a.writeJSONBody(w, buff)
}

func (a *API) saveHTTPRouteGroup(w http.ResponseWriter, r *http.Request, update bool) error {
	pbGroup := &v2alpha1.HTTPRouteGroup{}
	group := &spec.HTTPRouteGroup{}

	err := a.readAPISpec(r, pbGroup, group)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	name := group.Name

	a.service.Lock()
	defer a.service.Unlock()

	oldGroup := a.service.GetHTTPRouteGroup(name)
	if update && (oldGroup == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldGroup != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutHTTPRouteGroup(group)
	return nil
}

func (a *API) createHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	err := a.saveHTTPRouteGroup(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	a.saveHTTPRouteGroup(w, r, true)
}

func (a *API) deleteHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldGroup := a.service.GetHTTPRouteGroup(name)
	if oldGroup == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteHTTPRouteGroup(name)
}

func (a *API) listTrafficTargets(w http.ResponseWriter, r *http.Request) {
	tts := a.service.ListTrafficTargets()
	sort.Slice(tts, func(i, j int) bool {
		return tts[i].Name < tts[j].Name
	})

	pbTrafficTargets := make([]*v2alpha1.TrafficTarget, 0, len(tts))
	for _, v := range tts {
		tt := &v2alpha1.TrafficTarget{}
		err := a.convertSpecToPB(v, tt)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbTrafficTargets = append(pbTrafficTargets, tt)
	}

	buff := codectool.MustMarshalJSON(pbTrafficTargets)
	a.writeJSONBody(w, buff)
}

func (a *API) getTrafficTarget(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	tt := a.service.GetTrafficTarget(name)
	if tt == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbTrafficTarget := &v2alpha1.TrafficTarget{}
	err = a.convertSpecToPB(tt, pbTrafficTarget)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", tt, err))
	}

	buff := codectool.MustMarshalJSON(pbTrafficTarget)
	a.writeJSONBody(w, buff)
}

func (a *API) saveTrafficTarget(w http.ResponseWriter, r *http.Request, update bool) error {
	pbTrafficTarget := &v2alpha1.TrafficTarget{}
	tt := &spec.TrafficTarget{}

	err := a.readAPISpec(r, pbTrafficTarget, tt)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}
	name := tt.Name

	a.service.Lock()
	defer a.service.Unlock()

	oldTrafficTarget := a.service.GetTrafficTarget(name)
	if update && (oldTrafficTarget == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldTrafficTarget != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutTrafficTarget(tt)
	return nil
}

func (a *API) createTrafficTarget(w http.ResponseWriter, r *http.Request) {
	err := a.saveTrafficTarget(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateTrafficTarget(w http.ResponseWriter, r *http.Request) {
	a.saveTrafficTarget(w, r, true)
}

func (a *API) deleteTrafficTarget(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldTrafficTarget := a.service.GetTrafficTarget(name)
	if oldTrafficTarget == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteTrafficTarget(name)
}
