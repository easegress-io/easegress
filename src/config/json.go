package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"common"
	"logger"
	"pipelines"
	"plugins"
)

type GatewayPluginRepository struct {
	PluginSpecs []PluginSpec `json:"plugins"`
}

type GatewayPipelineRepository struct {
	PipelineSpecs []PipelineSpec `json:"pipelines"`
}

type JSONFileStore struct {
	// Fine-grained lock gives better I/O performance
	pluginLock   sync.RWMutex
	pipelineLock sync.RWMutex

	pluginFileFullPath   string
	pipelineFileFullPath string
}

func NewJSONFileStore() (*JSONFileStore, error) {
	pluginFile := "plugins.json"
	pipelineFile := "pipelines.json"

	if common.Stage == "test" {
		pluginFile = "plugins_test.json"
		pipelineFile = "pipelines_test.json"
	}

	pluginFullFile := filepath.Join(common.ConfigHome, pluginFile)
	pipelineFullFile := filepath.Join(common.ConfigHome, pipelineFile)

	logger.Debugf("[storage file path: %s]", pluginFullFile)
	logger.Debugf("[storage file path: %s]", pipelineFullFile)

	os.MkdirAll(filepath.Dir(pluginFullFile), 0700)
	os.MkdirAll(filepath.Dir(pipelineFullFile), 0700)

	for _, path := range []string{pluginFullFile, pipelineFullFile} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				msg := fmt.Sprintf(
					"[config:NewJSONFileStore] - [could not access storage file: %v]", err)
				logger.Errorf(msg)
				return nil, fmt.Errorf(msg)
			}
			file.Write([]byte("{}"))
			file.Close()

			logger.Debugf("[new storage file created: %v]", path)
		}
	}

	return &JSONFileStore{
		pluginFileFullPath:   pluginFullFile,
		pipelineFileFullPath: pipelineFullFile,
	}, nil
}

func (s *JSONFileStore) GetAllPlugins() ([]*PluginSpec, error) {
	logger.Debugf("[load all plugins]")

	var ret []*PluginSpec

	s.pluginLock.RLock()
	buff, err := ioutil.ReadFile(s.pluginFileFullPath)
	if err != nil {
		s.pluginLock.RUnlock()
		return nil, fmt.Errorf("read file %s failed: %v", s.pluginFileFullPath, err)
	}
	s.pluginLock.RUnlock()

	pluginRepo := new(GatewayPluginRepository)
	err = json.Unmarshal(buff, pluginRepo)
	if err != nil {
		return nil, err
	}

	for _, spec := range pluginRepo.PluginSpecs {
		constructor, e := plugins.GetConstructor(spec.Type)
		if e != nil {
			logger.Warnf("get plugin constructor failed, skipped: %v", e)
			continue
		}
		spec.Constructor = constructor
		s := spec // duplicate one
		ret = append(ret, &s)
	}

	return ret, nil
}

func (s *JSONFileStore) GetAllPipelines() ([]*PipelineSpec, error) {
	logger.Debugf("[load all pipelines]")

	var pipes []*PipelineSpec

	s.pipelineLock.RLock()
	buff, err := ioutil.ReadFile(s.pipelineFileFullPath)
	if err != nil {
		s.pipelineLock.RUnlock()
		return nil, fmt.Errorf("read file %s failed: %v", s.pipelineFileFullPath, err)
	}
	s.pipelineLock.RUnlock()

	pipelineRepo := new(GatewayPipelineRepository)
	err = json.Unmarshal(buff, pipelineRepo)
	if err != nil {
		return nil, err
	}

	for _, spec := range pipelineRepo.PipelineSpecs {
		s := spec // duplicate one
		pipes = append(pipes, &s)
	}

	return pipes, nil
}

func (s *JSONFileStore) AddPlugin(plugin *PluginSpec) error {
	conf, ok := plugin.Config.(plugins.Config)
	if !ok {
		return fmt.Errorf("invalid plugin config")
	}

	s.pluginLock.Lock()
	defer s.pluginLock.Unlock()

	logger.Debugf("[add plugin: %s]", conf.PluginName())

	buff, err := ioutil.ReadFile(s.pluginFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pluginFileFullPath, err)
	}

	pluginRepo := new(GatewayPluginRepository)
	err = json.Unmarshal(buff, pluginRepo)
	if err != nil {
		return err
	}

	for _, spec := range pluginRepo.PluginSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(plugins.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PluginName() == conf.PluginName() {
			return fmt.Errorf("duplicate plugin: %s", conf.PluginName())
		}
	}
	pluginRepo.PluginSpecs = append(pluginRepo.PluginSpecs, *plugin)

	err = s.saveJSONFile(pluginRepo, s.pluginFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) AddPipeline(pipeline *PipelineSpec) error {
	conf, ok := pipeline.Config.(pipelines.Config)
	if !ok {
		return fmt.Errorf("invalid pipeline config")
	}

	s.pipelineLock.Lock()
	defer s.pipelineLock.Unlock()

	logger.Debugf("[add pipeline: %s]", conf.PipelineName())

	buff, err := ioutil.ReadFile(s.pipelineFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pipelineFileFullPath, err)
	}

	pipelineRepo := new(GatewayPipelineRepository)
	err = json.Unmarshal(buff, pipelineRepo)
	if err != nil {
		return err
	}

	for _, spec := range pipelineRepo.PipelineSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(pipelines.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PipelineName() == conf.PipelineName() {
			return fmt.Errorf("duplicate pipeline: %s", conf.PipelineName())
		}
	}
	pipelineRepo.PipelineSpecs = append(pipelineRepo.PipelineSpecs, *pipeline)

	err = s.saveJSONFile(pipelineRepo, s.pipelineFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) DeletePlugin(name string) error {
	s.pluginLock.Lock()
	defer s.pluginLock.Unlock()

	logger.Debugf("[delete plugin: %s]", name)

	buff, err := ioutil.ReadFile(s.pluginFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pluginFileFullPath, err)
	}

	pluginRepo := new(GatewayPluginRepository)
	err = json.Unmarshal(buff, pluginRepo)
	if err != nil {
		return err
	}

	var newPluginSpecs []PluginSpec
	deleted := false
	for _, spec := range pluginRepo.PluginSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(plugins.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PluginName() != name {
			newPluginSpecs = append(newPluginSpecs, spec)
		} else {
			deleted = true
		}
	}

	if !deleted {
		return nil
	}

	pluginRepo.PluginSpecs = newPluginSpecs

	err = s.saveJSONFile(pluginRepo, s.pluginFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) DeletePipeline(name string) error {
	s.pipelineLock.Lock()
	defer s.pipelineLock.Unlock()

	logger.Debugf("[delete pipeline: %s]", name)

	buff, err := ioutil.ReadFile(s.pipelineFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pipelineFileFullPath, err)
	}

	pipelineRepo := new(GatewayPipelineRepository)
	err = json.Unmarshal(buff, pipelineRepo)
	if err != nil {
		return err
	}

	var newPipelineSpecs []PipelineSpec
	deleted := false
	for _, spec := range pipelineRepo.PipelineSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(pipelines.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PipelineName() != name {
			newPipelineSpecs = append(newPipelineSpecs, spec)
		} else {
			deleted = true
		}
	}

	if !deleted {
		return nil
	}

	pipelineRepo.PipelineSpecs = newPipelineSpecs

	err = s.saveJSONFile(pipelineRepo, s.pipelineFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) UpdatePlugin(plugin *PluginSpec) error {
	conf, ok := plugin.Config.(plugins.Config)
	if !ok {
		return fmt.Errorf("invalid plugin config")
	}

	s.pluginLock.Lock()
	defer s.pluginLock.Unlock()

	logger.Debugf("[update plugin: %s]", conf.PluginName())

	buff, err := ioutil.ReadFile(s.pluginFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pluginFileFullPath, err)
	}

	pluginRepo := new(GatewayPluginRepository)
	err = json.Unmarshal(buff, pluginRepo)
	if err != nil {
		return err
	}

	var newPluginSpecs []PluginSpec
	for _, spec := range pluginRepo.PluginSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(plugins.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PluginName() == conf.PluginName() {
			newPluginSpecs = append(newPluginSpecs, *plugin)
		} else {
			newPluginSpecs = append(newPluginSpecs, spec)
		}
	}
	pluginRepo.PluginSpecs = newPluginSpecs

	err = s.saveJSONFile(pluginRepo, s.pluginFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) UpdatePipeline(pipeline *PipelineSpec) error {
	conf, ok := pipeline.Config.(pipelines.Config)
	if !ok {
		return fmt.Errorf("invalid pipeline config")
	}

	s.pipelineLock.Lock()
	defer s.pipelineLock.Unlock()

	logger.Debugf("[update pipeline: %s]", conf.PipelineName())

	buff, err := ioutil.ReadFile(s.pipelineFileFullPath)
	if err != nil {
		return fmt.Errorf("read file %s failed: %v", s.pipelineFileFullPath, err)
	}

	pipelineRepo := new(GatewayPipelineRepository)
	err = json.Unmarshal(buff, pipelineRepo)
	if err != nil {
		return err
	}

	var newPipelineSpecs []PipelineSpec
	for _, spec := range pipelineRepo.PipelineSpecs {
		buff, _ := json.Marshal(spec.Config)
		c := new(pipelines.CommonConfig)
		json.Unmarshal(buff, c)

		if c.PipelineName() == conf.PipelineName() {
			newPipelineSpecs = append(newPipelineSpecs, *pipeline)
		} else {
			newPipelineSpecs = append(newPipelineSpecs, spec)
		}
	}
	pipelineRepo.PipelineSpecs = newPipelineSpecs

	err = s.saveJSONFile(pipelineRepo, s.pipelineFileFullPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *JSONFileStore) saveJSONFile(repo interface{}, filePath string) error {
	// Proper write lock should be controlled by outside caller.

	buff, err := json.Marshal(repo)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filePath, buff, 0600)
	if err != nil {
		logger.Errorf("[a corrupt storage file %s could remain due to %v]", filePath, err)
		return err
	} else {
		return nil
	}
}
