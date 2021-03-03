package api

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/v"

	"github.com/kataras/iris"
	yaml "gopkg.in/yaml.v2"
)

const (
	// MetadataPrefix is the metadata prefix.
	MetadataPrefix = "/metadata"

	// ObjectMetadataPrefix is the object metadata prefix.
	ObjectMetadataPrefix = "/metadata/objects"

	// FilterMetaPrefix is the filter of HTTPPipeline metadata prefix.
	FilterMetaPrefix = "/metadata/objects/httppipeline/filters"
)

type (
	// FilterMeta is the metadata of filter.
	FilterMeta struct {
		Kind        string
		Results     []string
		SpecType    reflect.Type
		Description string
	}
)

var (
	filterMetaBook = map[string]*FilterMeta{}
	filterKinds    []string
)

func (s *Server) setupMetadaAPIs() {
	filterRegistry := httppipeline.GetFilterRegistry()
	for kind, f := range filterRegistry {
		filterMetaBook[kind] = &FilterMeta{
			Kind:        kind,
			Results:     f.Results(),
			SpecType:    reflect.TypeOf(f.DefaultSpec()),
			Description: f.Description(),
		}
		filterKinds = append(filterKinds, kind)
		sort.Strings(filterMetaBook[kind].Results)
	}
	sort.Strings(filterKinds)

	metadataAPIs := make([]*APIEntry, 0)
	metadataAPIs = append(metadataAPIs,
		&APIEntry{
			Path:    FilterMetaPrefix,
			Method:  "GET",
			Handler: s.listFilters,
		},
		&APIEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/description",
			Method:  "GET",
			Handler: s.getFilterDescription,
		},
		&APIEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/schema",
			Method:  "GET",
			Handler: s.getFilterSchema,
		},
		&APIEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/results",
			Method:  "GET",
			Handler: s.getFilterResults,
		},
	)

	s.RegisterAPIs(metadataAPIs)
}

func (s *Server) listFilters(ctx iris.Context) {
	buff, err := yaml.Marshal(filterKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", filterKinds, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) getFilterDescription(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	ctx.WriteString(fm.Description)
}

func (s *Server) getFilterSchema(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := v.GetSchemaInYAML(fm.SpecType)
	if err != nil {
		panic(fmt.Errorf("get schema for %v failed: %v", fm.Kind, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) getFilterResults(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := yaml.Marshal(fm.Results)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", fm.Results, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}
