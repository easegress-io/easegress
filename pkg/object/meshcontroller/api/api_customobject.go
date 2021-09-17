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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easemesh-api/v1alpha1"
	"github.com/xeipuuv/gojsonschema"
)

func (a *API) readURLParam(r *http.Request, name string) (string, error) {
	value := chi.URLParam(r, name)
	if value == "" {
		return "", fmt.Errorf("required URL parameter %s is empty", name)
	}
	return value, nil
}

func (a *API) listCustomObjectKinds(w http.ResponseWriter, r *http.Request) {
	kinds := a.service.ListCustomObjectKinds()
	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i].Name < kinds[j].Name
	})

	var pbKinds []*v1alpha1.CustomObjectKind
	for _, v := range kinds {
		kind := &v1alpha1.CustomObjectKind{}
		err := a.convertSpecToPB(v, kind)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbKinds = append(pbKinds, kind)
	}

	err := json.NewEncoder(w).Encode(pbKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", kinds, err))
	}

	w.Header().Set("Content-Type", "application/json")
}

func (a *API) getCustomObjectKind(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	kind := a.service.GetCustomObjectKind(name)
	if kind == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbKind := &v1alpha1.CustomObjectKind{}
	err = a.convertSpecToPB(kind, pbKind)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", kind, err))
	}

	err = json.NewEncoder(w).Encode(pbKind)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbKind, err))
	}

	w.Header().Set("Content-Type", "application/json")
}

func (a *API) saveCustomObjectKind(w http.ResponseWriter, r *http.Request, update bool) error {
	pbKind := &v1alpha1.CustomObjectKind{}
	kind := &spec.CustomObjectKind{}

	err := a.readAPISpec(r, pbKind, kind)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	name := kind.Name
	if update {
		name, err = a.readURLParam(r, "name")
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}

		if name != kind.Name {
			err = fmt.Errorf("name conflict: %s %s", name, kind.Name)
			api.HandleAPIError(w, r, http.StatusConflict, err)
			return err
		}
	}

	if kind.JSONSchema != "" {
		sl := gojsonschema.NewStringLoader(kind.JSONSchema)
		if _, err = gojsonschema.NewSchema(sl); err != nil {
			err = fmt.Errorf("invalid JSONSchema: %s", err.Error())
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldKind := a.service.GetCustomObjectKind(name)
	if update && (oldKind == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldKind != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutCustomObjectKind(kind)
	return nil
}

func (a *API) createCustomObjectKind(w http.ResponseWriter, r *http.Request) {
	err := a.saveCustomObjectKind(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateCustomObjectKind(w http.ResponseWriter, r *http.Request) {
	a.saveCustomObjectKind(w, r, true)
}

func (a *API) deleteCustomObjectKind(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldKind := a.service.GetCustomObjectKind(name)
	if oldKind == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteCustomObjectKind(name)
	// TODO: remove custom objects
}

func (a *API) listAllCustomObjects(w http.ResponseWriter, r *http.Request) {
	objs := a.service.ListCustomObjects("")
	sort.Slice(objs, func(i, j int) bool {
		k1, k2 := objs[i].Kind(), objs[j].Kind()
		if k1 < k2 {
			return true
		}
		if k1 > k2 {
			return false
		}
		return objs[i].Name() < objs[j].Name()
	})

	err := json.NewEncoder(w).Encode(objs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", objs, err))
	}

	w.Header().Set("Content-Type", "application/json")
}

func (a *API) listCustomObjects(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	objs := a.service.ListCustomObjects(kind)
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Name() < objs[j].Name()
	})

	err = json.NewEncoder(w).Encode(objs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", objs, err))
	}

	w.Header().Set("Content-Type", "application/json")
}

func (a *API) getCustomObject(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	obj := a.service.GetCustomObject(kind, name)
	if obj == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	err = json.NewEncoder(w).Encode(obj)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", obj, err))
	}

	w.Header().Set("Content-Type", "application/json")
}

func (a *API) saveCustomObject(w http.ResponseWriter, r *http.Request, update bool) error {
	obj := &spec.CustomObject{}
	err := json.NewDecoder(r.Body).Decode(obj)
	if err != nil {
		err = fmt.Errorf("unmarshal custom object failed: %v", err)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	kind, name := obj.Kind(), obj.Name()
	if kind == "" {
		err = fmt.Errorf("kind cannot be empty")
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}
	if name == "" {
		err = fmt.Errorf("name cannot be empty")
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	if update {
		kind, err = a.readURLParam(r, "kind")
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
		name, err = a.readURLParam(r, "name")
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}

		if obj.Kind() != kind {
			err = fmt.Errorf("kind conflict: %s %s", kind, obj.Kind())
			api.HandleAPIError(w, r, http.StatusConflict, err)
			return err
		}

		if obj.Name() != name {
			err = fmt.Errorf("name conflict: %s %s", name, obj.Name())
			api.HandleAPIError(w, r, http.StatusConflict, err)
			return err
		}
	}

	k := a.service.GetCustomObjectKind(obj.Kind())
	if k == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("kind %s not found", kind))
		return err
	}

	if k.JSONSchema != "" {
		schema := gojsonschema.NewStringLoader(k.JSONSchema)
		doc := gojsonschema.NewGoLoader(obj)
		res, err := gojsonschema.Validate(schema, doc)
		if err != nil {
			err = fmt.Errorf("validation failed: %v", err)
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
		if !res.Valid() {
			err = fmt.Errorf("invalid custom object: %v", res.Errors())
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldObj := a.service.GetCustomObject(kind, name)
	if update && (oldObj == nil) {
		err = fmt.Errorf("custom object %s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldObj != nil) {
		err = fmt.Errorf("custom object %s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutCustomObject(obj)
	return nil
}

func (a *API) createCustomObject(w http.ResponseWriter, r *http.Request) {
	err := a.saveCustomObject(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateCustomObject(w http.ResponseWriter, r *http.Request) {
	a.saveCustomObject(w, r, true)
}

func (a *API) deleteCustomObject(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldObj := a.service.GetCustomObject(kind, name)
	if oldObj == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteCustomObject(kind, name)
}

func (a *API) watchCustomObjects(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	logger.Infof("begin watch custom objects of kind '%s'", kind)

	w.Header().Set("Content-type", "application/octet-stream")
	a.service.WatchCustomObject(r.Context(), kind, func(objs []*spec.CustomObject) {
		err = json.NewEncoder(w).Encode(objs)
		if err != nil {
			logger.Errorf("marshal custom object failed: %v", err)
		}
		w.Write([]byte("\r\n"))
		w.(http.Flusher).Flush()
	})

	logger.Infof("end watch custom objects of kind '%s'", kind)
}
