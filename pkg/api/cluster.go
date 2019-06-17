package api

import (
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/registry"
)

func (s *Server) _purgeMember(memberName string) {
	err := s.cluster.PurgeMember(memberName)
	if err != nil {
		clusterPanic(fmt.Errorf("purge member %s failed: %s", memberName, err))
	}
}

func (s *Server) _getObject(name string) registry.Spec {
	value, err := s.cluster.Get(s.cluster.Layout().ConfigObjectKey(name))
	if err != nil {
		clusterPanic(err)
	}

	if value == nil {
		return nil
	}

	spec, err := registry.SpecFromYAML(*value)
	if err != nil {
		panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, *value))
	}

	return spec
}

func (s *Server) _listObjects() []registry.Spec {
	kvs, err := s.cluster.GetPrefix(s.cluster.Layout().ConfigObjectPrefix())
	if err != nil {
		clusterPanic(err)
	}

	specs := make([]registry.Spec, 0, len(kvs))
	for _, v := range kvs {
		spec, err := registry.SpecFromYAML(v)
		if err != nil {
			panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, v))
		}
		specs = append(specs, spec)
	}

	return specs
}

func (s *Server) _putObject(spec registry.Spec) {
	err := s.cluster.Put(s.cluster.Layout().ConfigObjectKey(spec.GetName()),
		registry.YAMLFromSpec(spec))
	if err != nil {
		clusterPanic(err)
	}
}

func (s *Server) _deleteObject(name string) {
	err := s.cluster.Delete(s.cluster.Layout().ConfigObjectKey(name))
	if err != nil {
		clusterPanic(err)
	}
}

func (s *Server) _getObjectStatus(name string) map[string]string {
	prefix := s.cluster.Layout().StatusObjectPrefix(name)
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		clusterPanic(err)
	}

	statuses := make(map[string]string)
	for k, v := range kvs {
		statuses[strings.TrimPrefix(k, prefix)] = v
	}

	return statuses
}
