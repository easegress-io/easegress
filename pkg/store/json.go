package store

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/plugins"
)

type (
	jsonFileConfig struct {
		sync.RWMutex `json:"-"`
		Plugins      map[string]*PluginSpec   `json:"plugins"`
		Pipelines    map[string]*PipelineSpec `json:"pipelines"`
	}
	jsonFileStore struct {
		watchersLock sync.Mutex
		watchers     map[string]*Watcher
		diffChan     chan *DiffSpec

		configPath string
		config     *jsonFileConfig
	}
)

func newJSONFileStore() (*jsonFileStore, error) {
	configFile := "runtime_config.json"
	configPath := filepath.Join(option.Global.DataDir, configFile)
	logger.Debugf("[runtime config path: %s]", configPath)

	err := os.MkdirAll(filepath.Dir(configPath), 0700)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return nil, fmt.Errorf("open file %s failed: %v", configPath, err)
		}
		f.Write([]byte("{}"))
		f.Close()
	}

	buff, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read file %s failed: %v", configPath, err)
	}

	config := new(jsonFileConfig)
	err = json.Unmarshal(buff, config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s to json failed: %v", configPath, err)
	}

	s := &jsonFileStore{
		watchers:   make(map[string]*Watcher),
		diffChan:   make(chan *DiffSpec, 100),
		configPath: configPath,
		config: &jsonFileConfig{
			Plugins:   make(map[string]*PluginSpec),
			Pipelines: make(map[string]*PipelineSpec),
		},
	}

	// To validate existed config.
	for _, plugin := range config.Plugins {
		err := s.createPlugin(plugin)
		if err != nil {
			return nil, err
		}
	}
	for _, pipeline := range config.Pipelines {
		err := s.createPipeline(pipeline)
		if err != nil {
			return nil, err
		}
	}

	go s.notify()

	return s, nil
}

func (s *jsonFileStore) Close() {
	close(s.diffChan)
	logger.Infof("[diff channel closed]")
	s.watchersLock.Lock()
	for name, watcher := range s.watchers {
		watcher.close()
		logger.Infof("[watcher %s closed]", name)
	}
	s.watchersLock.Unlock()
}

func (s *jsonFileStore) notify() {
	for diffSpec := range s.diffChan {
		s.watchersLock.Lock()
		for name, watcher := range s.watchers {
			select {
			case <-time.After(100 * time.Millisecond):
				logger.Errorf("[send diff to watcher %s timeout(100ms), close it]", name)
				watcher.close()
				delete(s.watchers, name)
			case watcher.diffSpecChan <- diffSpec:
			}
		}
		s.watchersLock.Unlock()
	}
}

// flush doesn't care concurrent problem, it's callers' duty.
func (s *jsonFileStore) flush() error {
	buff, err := json.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("marshal config to json failed: %v", err)
	}

	err = ioutil.WriteFile(s.configPath, buff, 0600)
	if err != nil {
		return fmt.Errorf("write file %s failed: %v", s.configPath, err)
	}

	return nil
}

func (s *jsonFileStore) pipelineNames() []string {
	names := make([]string, 0, len(s.config.Pipelines))
	for name := range s.config.Pipelines {
		names = append(names, name)
	}

	return names
}

func (s *jsonFileStore) isPluginRunning(pluginName string) bool {
	for _, pipelineSpec := range s.config.Pipelines {
		if common.StrInSlice(pluginName, pipelineSpec.Config.Plugins) {
			return true
		}
	}

	return false
}

func (s *jsonFileStore) ListPluginPipelines() (map[string]*PluginSpec, map[string]*PipelineSpec) {
	s.config.RLock()
	defer s.config.RUnlock()

	plugins := make(map[string]*PluginSpec)
	for name, pluginSpec := range s.config.Plugins {
		plugins[name] = pluginSpec
	}

	pipelines := make(map[string]*PipelineSpec)
	for name, pipelineSpec := range s.config.Pipelines {
		pipelines[name] = pipelineSpec
	}

	return plugins, pipelines
}

func (s *jsonFileStore) ClaimWatcher(name string) (*Watcher, error) {
	if name == "" {
		return nil, fmt.Errorf("emtpy watcher name")
	}

	watcher := newWatcher()

	s.watchersLock.Lock()
	if _, exists := s.watchers[name]; exists {
		return nil, fmt.Errorf("conflict watcher name: %s", name)
	}
	s.watchers[name] = watcher
	s.watchersLock.Unlock()

	// NOTICE: It relys on the size of diffSpecChan is larger than zero.
	plugins, pipelines := s.ListPluginPipelines()
	watcher.diffSpecChan <- &DiffSpec{
		Total:                     true,
		CreatedOrUpdatedPlugins:   plugins,
		CreatedOrUpdatedPipelines: pipelines,
	}

	logger.Infof("[claimed watcher %s]", name)

	return watcher, nil
}

func (s *jsonFileStore) DeleteWatcher(name string) {
	s.watchersLock.Lock()
	defer s.watchersLock.Unlock()

	if watcher := s.watchers[name]; watcher != nil {
		watcher.close()
	}
	delete(s.watchers, name)
}

func (s *jsonFileStore) GetPlugin(name string) *PluginSpec {
	s.config.RLock()
	defer s.config.RUnlock()

	return s.config.Plugins[name]
}

func (s *jsonFileStore) GetPipeline(name string) *PipelineSpec {
	s.config.RLock()
	defer s.config.RUnlock()

	return s.config.Pipelines[name]
}

func (s *jsonFileStore) ListPlugins() map[string]*PluginSpec {
	s.config.RLock()
	defer s.config.RUnlock()

	plugins := make(map[string]*PluginSpec)
	for name, pluginSpec := range s.config.Plugins {
		plugins[name] = pluginSpec
	}

	return plugins
}

func (s *jsonFileStore) ListPipelines() map[string]*PipelineSpec {
	s.config.RLock()
	defer s.config.RUnlock()

	pipelines := make(map[string]*PipelineSpec)
	for name, pipelineSpec := range s.config.Pipelines {
		pipelines[name] = pipelineSpec
	}

	return pipelines
}

func (s *jsonFileStore) CreatePlugin(pluginSpec *PluginSpec) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.createPlugin(pluginSpec)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		CreatedOrUpdatedPlugins: map[string]*PluginSpec{
			pluginSpec.Name: pluginSpec,
		},
	}

	return nil
}

func (s *jsonFileStore) createPlugin(pluginSpec *PluginSpec) error {
	if err := common.ValidateName(pluginSpec.Name); err != nil {
		return err
	}
	if _, exists := s.config.Plugins[pluginSpec.Name]; exists {
		return fmt.Errorf("conflict name: %s", pluginSpec.Name)
	}

	constructor, config, err := plugins.GetConstructorConfig(pluginSpec.Type)
	if err != nil {
		return fmt.Errorf("get contructor and config for plugin %s(type: %s) failed: %v",
			pluginSpec.Name, pluginSpec.Type, err)
	}

	buff, err := json.Marshal(pluginSpec.Config)
	if err != nil {
		return fmt.Errorf("marshal %#v failed: %v", pluginSpec.Config, err)
	}
	err = json.Unmarshal(buff, config)
	if err != nil {
		return fmt.Errorf("unmarshal %s for config of plugin %s(type %s) failed: %v",
			buff, pluginSpec.Name, pluginSpec.Type, err)
	}
	err = config.Prepare(s.pipelineNames())
	if err != nil {
		return fmt.Errorf("prepare config for plugin %s(type %s) failed: %v",
			pluginSpec.Name, pluginSpec.Type, err)
	}

	pluginSpec.Constructor, pluginSpec.Config = constructor, config
	s.config.Plugins[pluginSpec.Name] = pluginSpec

	return nil
}

func (s *jsonFileStore) CreatePipeline(pipelineSpec *PipelineSpec) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.createPipeline(pipelineSpec)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		CreatedOrUpdatedPipelines: map[string]*PipelineSpec{
			pipelineSpec.Name: pipelineSpec,
		},
	}

	return nil
}

func (s *jsonFileStore) createPipeline(pipelineSpec *PipelineSpec) error {
	if err := common.ValidateName(pipelineSpec.Name); err != nil {
		return err
	}
	if _, exists := s.config.Pipelines[pipelineSpec.Name]; exists {
		return fmt.Errorf("conflict name: %s", pipelineSpec.Name)
	}

	err := pipelineSpec.Config.Prepare()
	if err != nil {
		return fmt.Errorf("prepare config for pipeline %s failed: %v",
			pipelineSpec.Name, err)
	}

	for _, pluginName := range pipelineSpec.Config.Plugins {
		if _, exists := s.config.Plugins[pluginName]; !exists {
			return fmt.Errorf("plugin %s not found", pluginName)
		}
	}

	s.config.Pipelines[pipelineSpec.Name] = pipelineSpec

	return nil
}

func (s *jsonFileStore) DeletePlugin(name string) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.deletePlugin(name)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		DeletedPlugins: []string{name},
	}

	return nil
}

func (s *jsonFileStore) deletePlugin(name string) error {
	if _, exists := s.config.Plugins[name]; !exists {
		return fmt.Errorf("not found plugin %s", name)
	}

	if s.isPluginRunning(name) {
		return fmt.Errorf("plugin is running")
	}

	delete(s.config.Plugins, name)

	return nil
}

func (s *jsonFileStore) DeletePipeline(name string) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.deletePipeline(name)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		DeletedPipelines: []string{name},
	}

	return nil
}

func (s *jsonFileStore) deletePipeline(name string) error {
	if _, exists := s.config.Pipelines[name]; !exists {
		return fmt.Errorf("not found pipeline %s", name)
	}

	return nil
}

func (s *jsonFileStore) UpdatePlugin(pluginSpec *PluginSpec) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.updatePlugin(pluginSpec)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		CreatedOrUpdatedPlugins: map[string]*PluginSpec{
			pluginSpec.Name: pluginSpec,
		},
	}

	return nil
}

func (s *jsonFileStore) updatePlugin(pluginSpec *PluginSpec) error {
	oldPluginSpec, exists := s.config.Plugins[pluginSpec.Name]
	if !exists {
		return fmt.Errorf("not found plugin %s", pluginSpec.Name)
	}

	if oldPluginSpec.Type != pluginSpec.Type {
		return fmt.Errorf("type of plugin %s is %s, changed to %s",
			pluginSpec.Name, oldPluginSpec.Type, pluginSpec.Type)
	}

	constructor, config, err := plugins.GetConstructorConfig(pluginSpec.Type)
	if err != nil {
		return fmt.Errorf("get contructor and config for plugin %s(type: %s) failed: %v",
			pluginSpec.Name, pluginSpec.Type, err)
	}

	buff, err := json.Marshal(pluginSpec.Config)
	if err != nil {
		return fmt.Errorf("marshal %#v failed: %v", pluginSpec.Config, err)
	}
	err = json.Unmarshal(buff, config)
	if err != nil {
		return fmt.Errorf("unmarshal %s for config of plugin %s(type %s) failed: %v",
			buff, pluginSpec.Name, pluginSpec.Type, err)
	}
	err = config.Prepare(s.pipelineNames())
	if err != nil {
		return fmt.Errorf("prepare config for plugin %s(type %s) failed: %v",
			pluginSpec.Name, pluginSpec.Type, err)
	}

	pluginSpec.Constructor, pluginSpec.Config = constructor, config
	s.config.Plugins[pluginSpec.Name] = pluginSpec

	return nil
}

func (s *jsonFileStore) UpdatePipeline(pipelineSpec *PipelineSpec) error {
	s.config.Lock()
	defer s.config.Unlock()

	err := s.updatePipeline(pipelineSpec)
	if err != nil {
		return err
	}

	s.flush()
	s.diffChan <- &DiffSpec{
		CreatedOrUpdatedPipelines: map[string]*PipelineSpec{
			pipelineSpec.Name: pipelineSpec,
		},
	}

	return nil
}

func (s *jsonFileStore) updatePipeline(pipelineSpec *PipelineSpec) error {
	oldPipelineSpec, exists := s.config.Pipelines[pipelineSpec.Name]
	if !exists {
		return fmt.Errorf("not found pipeline %s", pipelineSpec.Name)
	}

	if oldPipelineSpec.Type != pipelineSpec.Type {
		return fmt.Errorf("type of pipeline %s is %s, changed to %s",
			pipelineSpec.Name, oldPipelineSpec.Type, pipelineSpec.Type)
	}

	err := pipelineSpec.Config.Prepare()
	if err != nil {
		return fmt.Errorf("prepare config for pipeline %s failed: %v",
			pipelineSpec.Name, err)
	}

	for _, pluginName := range pipelineSpec.Config.Plugins {
		if _, exists := s.config.Plugins[pluginName]; !exists {
			return fmt.Errorf("plugin %s not found", pluginName)
		}
	}

	s.config.Pipelines[pipelineSpec.Name] = pipelineSpec

	return nil
}

func (s *jsonFileStore) adaptTotalDiffSpec(diffSpec *DiffSpec) {
	if !diffSpec.Total {
		return
	}

	for _, oldPluginSpec := range s.config.Plugins {
		pluginSpec, exists := diffSpec.CreatedOrUpdatedPlugins[oldPluginSpec.Name]
		if !exists {
			diffSpec.DeletedPlugins = append(diffSpec.DeletedPlugins,
				oldPluginSpec.Name)
			continue
		}
		if oldPluginSpec.equal(pluginSpec) {
			delete(diffSpec.CreatedOrUpdatedPlugins, pluginSpec.Name)
		}
	}

	for _, oldPipelineSpec := range s.config.Pipelines {
		pipelineSpec, exists := diffSpec.CreatedOrUpdatedPipelines[oldPipelineSpec.Name]
		if !exists {
			diffSpec.DeletedPipelines = append(diffSpec.DeletedPipelines,
				oldPipelineSpec.Name)
			continue
		}
		if oldPipelineSpec.equal(pipelineSpec) {
			delete(diffSpec.CreatedOrUpdatedPipelines, pipelineSpec.Name)
		}
	}

	diffSpec.Total = false
}

func (s *jsonFileStore) ApplyDiff(diffSpec *DiffSpec) error {
	s.config.Lock()
	defer s.config.Unlock()

	s.adaptTotalDiffSpec(diffSpec)

	for _, pluginSpec := range diffSpec.CreatedOrUpdatedPlugins {
		createErr := s.createPlugin(pluginSpec)
		if createErr != nil {
			updateErr := s.updatePlugin(pluginSpec)
			if updateErr != nil {
				logger.Errorf("apply plugin failed: "+
					"create: %v | update: %v", createErr, updateErr)
				continue
			}
		}
	}

	for _, pipelineSpec := range diffSpec.CreatedOrUpdatedPipelines {
		createErr := s.createPipeline(pipelineSpec)
		if createErr != nil {
			updateErr := s.updatePipeline(pipelineSpec)
			if updateErr != nil {
				logger.Errorf("apply pipeline failed: "+
					"create: %v | update:%v", createErr, updateErr)
				continue
			}
		}
	}

	for _, name := range diffSpec.DeletedPipelines {
		err := s.deletePipeline(name)
		if err != nil {
			logger.Errorf("delete pipeline failed: %v", err)
			continue
		}
	}

	for _, name := range diffSpec.DeletedPlugins {
		err := s.deletePlugin(name)
		if err != nil {
			logger.Errorf("delete plugin failed: %v", err)
			continue
		}
	}

	s.flush()
	s.diffChan <- diffSpec

	return nil
}
