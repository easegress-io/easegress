package api

import (
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/registry"

	yaml "gopkg.in/yaml.v2"
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

func (s *Server) _getStatusObject(name string) map[string]string {
	prefix := s.cluster.Layout().StatusObjectPrefix(name)
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		clusterPanic(err)
	}

	status := make(map[string]string)
	for k, v := range kvs {
		// NOTE: Here omitting the step yaml.Unmarshal in _listStatusObjects.
		status[strings.TrimPrefix(k, prefix)] = v
	}

	return status
}

func (s *Server) _listStatusObjects() map[string]map[string]interface{} {
	prefix := s.cluster.Layout().StatusObjectsPrefix()
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		clusterPanic(err)
	}

	status := make(map[string]map[string]interface{})
	for k, v := range kvs {
		k = strings.TrimPrefix(k, prefix)

		om := strings.Split(k, "/")
		if len(om) != 2 {
			clusterPanic(fmt.Errorf("the key %s can't be split into two fields by /", k))
		}
		objectName, memberName := om[0], om[1]
		_, exists := status[objectName]
		if !exists {
			status[objectName] = make(map[string]interface{})
		}

		// NOTE: This needs top-level of the status to be a map.
		i := map[string]interface{}{}
		err = yaml.Unmarshal([]byte(v), &i)
		if err != nil {
			clusterPanic(fmt.Errorf("unmarshal %s to yaml failed: %v", v, err))
		}
		status[objectName][memberName] = i
	}

	return status
}
