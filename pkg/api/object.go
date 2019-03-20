package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"

	// Ensure registered.
	_ "github.com/megaease/easegateway/pkg/object/httpproxy"
	_ "github.com/megaease/easegateway/pkg/object/httpserver"

	"github.com/megaease/easegateway/pkg/registry"

	"github.com/kataras/iris"
	yaml "gopkg.in/yaml.v2"
)

const (
	ObjectPrefix      = "/objects"
	ObjectKindsPrefix = "/object-kinds"
)

func (s *Server) setupObjectAPIs() {
	objAPIs := make([]*apiEntry, 0)
	objAPIs = append(objAPIs,
		&apiEntry{
			Path:    ObjectKindsPrefix,
			Method:  "GET",
			Handler: s.listObjectKinds,
		},

		&apiEntry{
			Path:    ObjectPrefix,
			Method:  "POST",
			Handler: s.createObject,
		},
		&apiEntry{
			Path:    ObjectPrefix,
			Method:  "GET",
			Handler: s.listObjects,
		},

		&apiEntry{
			Path:    ObjectPrefix + "/{name:string}",
			Method:  "GET",
			Handler: s.getObject,
		},
		&apiEntry{
			Path:    ObjectPrefix + "/{name:string}",
			Method:  "PUT",
			Handler: s.updateObject,
		},
		&apiEntry{
			Path:    ObjectPrefix + "/{name:string}",
			Method:  "DELETE",
			Handler: s.deleteObject,
		},

		&apiEntry{
			Path:    ObjectPrefix + "/{name:string}/status",
			Method:  "GET",
			Handler: s.getObjectStatus,
		},
	)

	s.apis = append(s.apis, objAPIs...)
}

func (s *Server) readObjectSpec(ctx iris.Context) (registry.Spec, error) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	spec, err := registry.SpecFromYAML(string(body))
	if err != nil {
		return nil, err
	}

	name := ctx.Params().Get("name")

	if name != "" && name != spec.GetName() {
		return nil, fmt.Errorf("inconsistent name in url and spec ")
	}

	return spec, err
}

func (s *Server) createObject(ctx iris.Context) {
	spec, err := s.readObjectSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	name := spec.GetName()

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getObject(name)
	if existedSpec != nil {
		handleAPIError(ctx, iris.StatusConflict, fmt.Errorf("conflict name"))
		return
	}

	s._putObject(spec)

	ctx.StatusCode(iris.StatusCreated)
	location := fmt.Sprintf("%s/%s", ctx.Path(), name)
	ctx.Header("Location", location)
}

func (s *Server) deleteObject(ctx iris.Context) {
	name := ctx.Params().Get("name")

	s.Lock()
	defer s.Unlock()

	spec := s._getObject(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	s._deleteObject(name)
}

func (s *Server) getObject(ctx iris.Context) {
	name := ctx.Params().Get("name")

	// No need to lock.

	spec := s._getObject(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	// Reference: https://mailarchive.ietf.org/arch/msg/media-types/e9ZNC0hDXKXeFlAVRWxLCCaG9GI
	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write([]byte(registry.YAMLFromSpec(spec)))
}

func (s *Server) updateObject(ctx iris.Context) {
	spec, err := s.readObjectSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	name := spec.GetName()

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getObject(name)
	if existedSpec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	if existedSpec.GetKind() != spec.GetKind() {
		handleAPIError(ctx, iris.StatusBadRequest,
			fmt.Errorf("different kinds: %s, %s",
				existedSpec.GetKind(), spec.GetKind()))
		return
	}

	s._putObject(spec)
}

func (s *Server) listObjects(ctx iris.Context) {
	// No need to lock.

	specs := s._listObjects()
	// NOTICE: Keep it consistent.
	sort.Sort(specsToSort(specs))

	buff := bytes.NewBuffer(nil)
	for _, spec := range specs {
		buff.WriteString(registry.YAMLFromSpec(spec))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff.Bytes())
}

func (s *Server) getObjectStatus(ctx iris.Context) {
	name := ctx.Params().Get("name")

	spec := s._getObject(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	// NOTICE: Maybe inconsistent, the object was deleted already here.
	statuses := s._getObjectStatus(name)

	buff, err := yaml.Marshal(statuses)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", statuses, err))
	}

	ctx.Write(buff)
}

type specsToSort []registry.Spec

func (s specsToSort) Less(i, j int) bool { return s[i].GetName() < s[j].GetName() }
func (s specsToSort) Len() int           { return len(s) }
func (s specsToSort) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *Server) listObjectKinds(ctx iris.Context) {
	var objKinds []string
	for _, obj := range registry.Objects() {
		objKinds = append(objKinds, obj.Kind())
	}

	// NOTICE: Keep it consistent.
	sort.Strings(objKinds)

	buff, err := yaml.Marshal(objKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", objKinds, err))
	}

	ctx.Write(buff)
}
