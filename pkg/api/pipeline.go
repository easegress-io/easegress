package api

import (
	"github.com/kataras/iris"
)

func pipelineAPIs(s *APIServer) []apiEntry {
	return []apiEntry{
		{
			path:    "/pipelines",
			method:  "POST",
			handler: s.createPipeline,
		},
		{
			path:    "/pipelines/{name:string}",
			method:  "PUT",
			handler: s.updatePipeline,
		},
		{
			path:    "/pipelines/{name:string}",
			method:  "GET",
			handler: s.getPipeline,
		},
		{
			path:    "/pipelines/{name:string}",
			method:  "DELETE",
			handler: s.deletePipeline,
		},
		{
			path:    "/pipelines",
			method:  "GET",
			handler: s.listPipelines,
		},
	}
}

func (s *APIServer) createPipeline(ctx iris.Context) {}
func (s *APIServer) deletePipeline(ctx iris.Context) {}
func (s *APIServer) getPipeline(ctx iris.Context)    {}
func (s *APIServer) updatePipeline(ctx iris.Context) {}
func (s *APIServer) listPipelines(ctx iris.Context)  {}
