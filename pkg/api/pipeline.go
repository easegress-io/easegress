package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/store"
)

func (s *APIServer) setupPipelineAPIs() {
	pipelineAPIs := []*apiEntry{
		{
			Path:    "/pipelines",
			Method:  "POST",
			Handler: s.createPipeline,
		},
		{
			Path:    "/pipelines/{name:string}",
			Method:  "PUT",
			Handler: s.updatePipeline,
		},
		{
			Path:    "/pipelines/{name:string}",
			Method:  "GET",
			Handler: s.getPipeline,
		},
		{
			Path:    "/pipelines/{name:string}",
			Method:  "DELETE",
			Handler: s.deletePipeline,
		},
		{
			Path:    "/pipelines",
			Method:  "GET",
			Handler: s.listPipelines,
		},
	}
	s.apis = append(s.apis, pipelineAPIs...)
}

func (s *APIServer) readPipelineSpec(ctx iris.Context) (*store.PipelineSpec, error) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	return store.NewPipelineSpec(string(body))
}

func (s *APIServer) createPipeline(ctx iris.Context) {
	spec, err := s.readPipelineSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	if err := common.ValidateName(spec.Name); err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getPipeline(spec.Name)
	if existedSpec != nil {
		handleAPIError(ctx, iris.StatusConflict, fmt.Errorf("conflict name"))
		return
	}

	err = spec.Bootstrap(s._pluginNames())
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s._putPipeline(nil, spec)

	ctx.StatusCode(iris.StatusCreated)
	location := fmt.Sprintf("%s/%s", ctx.Path(), spec.Name)
	ctx.Header("Location", location)
}

func (s *APIServer) deletePipeline(ctx iris.Context) {
	name := ctx.Params().Get("name")

	s.Lock()
	defer s.Unlock()

	spec := s._getPipeline(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	s._deletePipeline(spec)
}

func (s *APIServer) getPipeline(ctx iris.Context) {
	name := ctx.Params().Get("name")

	// No need to lock.

	spec := s._getPipeline(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", spec, err))
	}

	ctx.Write(buff)
}

func (s *APIServer) updatePipeline(ctx iris.Context) {
	name := ctx.Params().Get("name")

	spec, err := s.readPipelineSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	if name != spec.Name {
		handleAPIError(ctx, iris.StatusBadRequest,
			fmt.Errorf("spec got different name with url parameter"))
		return
	}

	s.Lock()
	defer s.Unlock()

	oldSpec := s._getPipeline(name)
	if oldSpec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	if oldSpec.Type != spec.Type {
		handleAPIError(ctx, iris.StatusBadRequest,
			fmt.Errorf("type of pipeline %s is %s, changed to %s",
				spec.Name, oldSpec.Type, spec.Type))
	}

	err = spec.Bootstrap(s._pluginNames())
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s._putPipeline(oldSpec, spec)
}

func (s *APIServer) listPipelines(ctx iris.Context) {
	// No need to lock.

	pipelines := s._listPipelines()

	buff, err := json.Marshal(pipelines)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pipelines, err))
	}

	ctx.Write(buff)
}
