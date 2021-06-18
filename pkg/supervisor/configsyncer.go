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

package supervisor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
)

const (
	configfileName = "running_objects.yaml"

	pullAllConfigTimeout = 1 * time.Minute
	rewatchTimeout       = 5 * time.Second
)

type (
	configSyncer struct {
		cls        cluster.Cluster
		watcher    cluster.Watcher
		prefix     string
		prefixChan <-chan map[string]*string

		configfilePath string
		config         map[string]string
		configChan     chan map[string]string

		first bool
		done  chan struct{}
	}
)

func newconfigSyncer(opt *option.Options, cls cluster.Cluster) *configSyncer {
	cs := &configSyncer{
		cls:    cls,
		prefix: cls.Layout().ConfigObjectPrefix(),

		configfilePath: filepath.Join(opt.AbsHomeDir, configfileName),
		config:         make(map[string]string),
		configChan:     make(chan map[string]string, 10),

		first: true,
		done:  make(chan struct{}),
	}

	var deltas []map[string]*string
	config, err := cs.loadConfig()
	if err != nil {
		logger.Errorf("load config failed: %v", err)
	} else {
		deltas = append(deltas, cs.configToDelta(config))
	}

	// Local file, then Etcd server.
	deltas = append(deltas, nil)
	cs.pullConfig(deltas...)

	cs.watchPrefixIfNeed()

	go cs.poll()

	return cs
}

// watchConfig return the channel to watch the change of config,
// the channel always output full config.
func (cs *configSyncer) watchConfig() <-chan map[string]string {
	return cs.configChan
}

func (cs *configSyncer) poll() {
	nextPullAllConfig := time.NewTicker(pullAllConfigTimeout)
	defer nextPullAllConfig.Stop()

	nextRewatchTimeout := time.NewTicker(rewatchTimeout)
	defer nextRewatchTimeout.Stop()

	for {
		select {
		case <-cs.done:
			cs.closeWatcher()
			return
		case <-nextRewatchTimeout.C:
			cs.watchPrefixIfNeed()
		case <-nextPullAllConfig.C:
			cs.pullConfig(nil)
		case delta, ok := <-cs.prefixChan:
			if ok {
				cs.pullConfig(delta)
			} else {
				cs.handleWatchFailed()
			}
		}
	}
}

// pullConfig applies deltas to the config by order.
// If the element delta is nil, it pulls all config
// from remote configSyncer.
func (cs *configSyncer) pullConfig(deltas ...map[string]*string) {
	newConfig := make(map[string]string)

	deltasCount := 0
	for _, delta := range deltas {
		if len(delta) == 0 {
			kvs, err := cs.cls.GetPrefix(cs.prefix)
			if err != nil {
				logger.Errorf("pull config failed: %v", err)
				continue
			}
			for k, v := range kvs {
				k = strings.TrimPrefix(k, cs.prefix)
				newConfig[k] = v
			}
		} else {
			newConfig = cs.copyConfig()
			for k, v := range delta {
				k = strings.TrimPrefix(k, cs.prefix)
				if v == nil {
					delete(newConfig, k)
					continue
				}
				newConfig[k] = *v
			}
		}
		deltasCount++
	}

	if deltasCount == 0 || reflect.DeepEqual(cs.config, newConfig) {
		// s.first for graceful update new process first run,
		// even deltasCount is zero, also need set firstDone to notify all objects ready
		if cs.first {
			cs.first = false
			cs.configChan <- cs.copyConfig()
		}
		return
	}

	cs.config = newConfig
	cs.configChan <- cs.copyConfig()
	cs.storeConfig()
}

func (cs *configSyncer) watchPrefixIfNeed() {
	if cs.watcher != nil {
		return
	}

	var (
		watcher    cluster.Watcher
		prefixChan <-chan map[string]*string
		err        error
	)

	defer func() {
		if err != nil {
			cs.handleWatchFailed()
		}
	}()

	watcher, err = cs.cls.Watcher()
	if err != nil {
		logger.Errorf("get watcher failed: %v", err)
		return
	}
	cs.watcher = watcher

	prefixChan, err = cs.watcher.WatchPrefix(cs.prefix)
	if err != nil {
		logger.Errorf("watch prefix failed: %v", err)
		return
	}
	cs.prefixChan = prefixChan
}

func (cs *configSyncer) closeWatcher() {
	cs.handleWatchFailed()
}

func (cs *configSyncer) handleWatchFailed() {
	if cs.watcher != nil {
		cs.watcher.Close()
	}
	cs.watcher, cs.prefixChan = nil, nil
}

func (cs *configSyncer) configToDelta(config map[string]string) map[string]*string {
	delta := make(map[string]*string)
	for k, v := range config {
		k = cs.prefix + k
		value := v
		delta[k] = &value
	}

	return delta
}

// loadConfig loads config from the local file.
func (cs *configSyncer) loadConfig() (map[string]string, error) {
	if _, err := os.Stat(cs.configfilePath); os.IsNotExist(err) {
		// NOTE: This is not an error.
		logger.Infof("%s not exist", cs.configfilePath)
		return nil, nil
	}

	buff, err := ioutil.ReadFile(cs.configfilePath)
	if err != nil {
		return nil, fmt.Errorf("%s read failed: %v", cs.configfilePath, err)
	}

	config := make(map[string]string)
	err = yaml.Unmarshal(buff, &config)
	if err != nil {
		return nil, fmt.Errorf("%s unmarshal to yaml failed: %v", cs.configfilePath, err)
	}

	return config, nil
}

func (cs *configSyncer) storeConfig() {
	buff := bytes.NewBuffer(nil)
	buff.WriteString(fmt.Sprintf("# %s\n", time.Now().Format(time.RFC3339)))

	configBuff, err := yaml.Marshal(cs.config)
	if err != nil {
		logger.Errorf("marshal %s to yaml failed: %v", buff, err)
		return
	}
	buff.Write(configBuff)

	err = ioutil.WriteFile(cs.configfilePath, buff.Bytes(), 0o644)
	if err != nil {
		logger.Errorf("write %s failed: %v", cs.configfilePath, err)
		return
	}
}

func (cs *configSyncer) copyConfig() map[string]string {
	config := make(map[string]string)
	for k, v := range cs.config {
		config[k] = v
	}
	return config
}

func (cs *configSyncer) close() {
	close(cs.done)
}
