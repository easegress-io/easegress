package store

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

const (
	watchTimeout = 10 * time.Second
)

type (
	jsonFileConfig struct {
		Plugins   map[string]*PluginSpec   `json:"plugins"`
		Pipelines map[string]*PipelineSpec `json:"pipelines"`
	}
	jsonFileStore struct {
		cluster     cluster.Cluster
		clusterChan <-chan map[string]*string
		eventChan   chan *Event
		closed      int32

		configPath string
		config     *jsonFileConfig
	}
)

func newJSONFileStore(c cluster.Cluster) (*jsonFileStore, error) {
	configFile := "runtime_config.json"
	configPath := filepath.Join(option.Global.DataDir, configFile)
	logger.Debugf("runtime config path: %s", configPath)

	err := os.MkdirAll(filepath.Dir(configPath), 0700)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			return nil, fmt.Errorf("open file %s failed: %v", configPath, err)
		}
		f.Write([]byte("{}"))
		f.Close()
	}

	clusterChan, err := c.WatchPrefix(cluster.ConfigObjectPrefix)
	if err != nil {
		return nil, fmt.Errorf("watch config objects failed: %v", err)
	}

	s := &jsonFileStore{
		cluster:     c,
		clusterChan: clusterChan,
		eventChan:   make(chan *Event, 10),
		configPath:  configPath,
		config: &jsonFileConfig{
			Plugins:   make(map[string]*PluginSpec),
			Pipelines: make(map[string]*PipelineSpec),
		},
	}

	go s.notify()

	return s, nil
}

func (s *jsonFileStore) Watch() <-chan *Event {
	return s.eventChan
}

func (s *jsonFileStore) notify() {
	for {
		kvs, ok := <-s.clusterChan
		if !ok {
			for {
				if atomic.LoadInt32(&s.closed) == 1 {
					return
				}
				clusterChan, err := s.cluster.WatchPrefix(cluster.ConfigObjectPrefix)
				if err != nil {
					logger.Errorf("watch config objects failed(retry after %v): %v",
						watchTimeout, err)
					<-time.After(watchTimeout)
				}
				s.clusterChan = clusterChan
			}
		}
		s.kvsToEvent(kvs)
		s.flush()
	}
}

func (s *jsonFileStore) kvsToEvent(kvs map[string]*string) {
	event := &Event{
		Plugins:   make(map[string]*PluginSpec),
		Pipelines: make(map[string]*PipelineSpec),
	}
	for k, v := range kvs {
		if strings.HasPrefix(k, cluster.ConfigPluginPrefix) {
			name := strings.TrimPrefix(k, cluster.ConfigPluginPrefix)
			if v == nil {
				delete(s.config.Plugins, name)
				event.Plugins[name] = nil
				continue
			}
			spec, err := NewPluginSpec(*v)
			if err != nil {
				logger.Errorf("new plugin spec for %s failed: %v", name, err)
				continue
			}
			if name != spec.Name {
				logger.Errorf("plugin %s got spec with differnt name %s",
					name, spec.Name)
				continue
			}
			s.config.Plugins[name], event.Plugins[name] = spec, spec
		} else if strings.HasPrefix(k, cluster.ConfigPipelinePrefix) {
			name := strings.TrimPrefix(k, cluster.ConfigPipelinePrefix)
			if v == nil {
				delete(s.config.Pipelines, name)
				event.Pipelines[name] = nil
				continue
			}
			spec, err := NewPipelineSpec(*v)
			if err != nil {
				logger.Errorf("new pipeline spec for %s failed: %v", name, err)
				continue
			}
			if name != spec.Name {
				logger.Errorf("pipeline %s got spec with differnt name %s",
					name, spec.Name)
				continue
			}
			s.config.Pipelines[name], event.Pipelines[name] = spec, spec
		} else {
			logger.Errorf("get unexpected config key: %s", k)
		}
	}

	for _, spec := range event.Plugins {
		if spec != nil {
			err := spec.Bootstrap(s.pipelineNames())
			if err != nil {
				logger.Errorf("plugin %s bootstrap failed: %v",
					spec.Name, err)
			}
		}
	}

	for _, spec := range s.config.Pipelines {
		if spec != nil {
			err := spec.Bootstrap(s.pluginNames())
			if err != nil {
				logger.Errorf("pipeline %s bootstrap failed: %v",
					spec.Name, err)
			}
		}
	}

	s.eventChan <- event
}

// flush doesn't care concurrent problem, it's callers' duty.
func (s *jsonFileStore) flush() error {
	buff, err := json.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("marshal config to json failed: %v", err)
	}

	err = ioutil.WriteFile(s.configPath, buff, 0640)
	if err != nil {
		return fmt.Errorf("write file %s failed: %v", s.configPath, err)
	}

	return nil
}

func (s *jsonFileStore) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	atomic.StoreInt32(&s.closed, 1)
	close(s.eventChan)
	logger.Infof("store closed")
}

func (s *jsonFileStore) pipelineNames() []string {
	names := make([]string, 0, len(s.config.Pipelines))
	for name := range s.config.Pipelines {
		names = append(names, name)
	}

	return names
}

func (s *jsonFileStore) pluginNames() map[string]struct{} {
	names := make(map[string]struct{})
	for name := range s.config.Plugins {
		names[name] = struct{}{}
	}
	return names
}
