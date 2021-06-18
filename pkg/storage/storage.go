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

package storage

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
	// Storage is the system component to interact with local and remote storage.
	Storage struct {
		cls        cluster.Cluster
		watcher    cluster.Watcher
		prefix     string
		prefixChan <-chan map[string]*string

		configfilePath string
		config         map[string]string
		configChan     chan map[string]string

		statusChan     chan map[string]string
		statusToDelete map[string]struct{}

		first bool
		done  chan struct{}
	}
)

// New creates a Storage.
func New(opt *option.Options, cls cluster.Cluster) *Storage {
	s := &Storage{
		cls:    cls,
		prefix: cls.Layout().ConfigObjectPrefix(),

		configfilePath: filepath.Join(opt.AbsHomeDir, configfileName),
		config:         make(map[string]string),
		configChan:     make(chan map[string]string, 10),

		statusChan:     make(chan map[string]string, 10),
		statusToDelete: make(map[string]struct{}),

		first: true,
		done:  make(chan struct{}),
	}

	var deltas []map[string]*string
	config, err := s.loadConfig()
	if err != nil {
		logger.Errorf("load config failed: %v", err)
	} else {
		deltas = append(deltas, s.configToDelta(config))
	}

	// Local file, then Etcd server.
	deltas = append(deltas, nil)
	s.pullConfig(deltas...)

	s.watchPrefixIfNeed()

	go s.poll()

	return s
}

// WatchConfig return the channel to watch the change of config,
// the channel always output full config.
func (s *Storage) WatchConfig() <-chan map[string]string {
	return s.configChan
}

// SyncStatus synchronizes the status.
func (s *Storage) SyncStatus(statuses map[string]string) {
	s.statusChan <- statuses
}

func (s *Storage) poll() {
	nextPullAllConfig := time.NewTicker(pullAllConfigTimeout)
	defer nextPullAllConfig.Stop()

	nextRewatchTimeout := time.NewTicker(rewatchTimeout)
	defer nextRewatchTimeout.Stop()

	for {
		select {
		case <-s.done:
			s.closeWatcher()
			return
		case <-nextRewatchTimeout.C:
			s.watchPrefixIfNeed()
		case statuses := <-s.statusChan:
			s.handlSyncStatus(statuses)
		case <-nextPullAllConfig.C:
			s.pullConfig(nil)
		case delta, ok := <-s.prefixChan:
			if ok {
				s.pullConfig(delta)
			} else {
				s.handleWatchFailed()
			}
		}
	}
}

// pullConfig applies deltas to the config by order.
// If the element delta is nil, it pulls all config
// from remote Storage.
func (s *Storage) pullConfig(deltas ...map[string]*string) {
	newConfig := make(map[string]string)

	deltasCount := 0
	for _, delta := range deltas {
		if len(delta) == 0 {
			kvs, err := s.cls.GetPrefix(s.prefix)
			if err != nil {
				logger.Errorf("pull config failed: %v", err)
				continue
			}
			for k, v := range kvs {
				k = strings.TrimPrefix(k, s.prefix)
				newConfig[k] = v
			}
		} else {
			newConfig = s.copyConfig()
			for k, v := range delta {
				k = strings.TrimPrefix(k, s.prefix)
				if v == nil {
					delete(newConfig, k)
					continue
				}
				newConfig[k] = *v
			}
		}
		deltasCount++
	}

	if deltasCount == 0 || reflect.DeepEqual(s.config, newConfig) {
		// s.first for graceful update new process first run,
		// even deltasCount is zero, also need set firstDone to notify all objects ready
		if s.first {
			s.first = false
			s.configChan <- s.copyConfig()
		}
		return
	}

	for k := range s.config {
		if _, exists := newConfig[k]; !exists {
			s.statusToDelete[k] = struct{}{}
		}
	}
	s.config = newConfig
	s.configChan <- s.copyConfig()
	s.storeConfig()
}

func (s *Storage) watchPrefixIfNeed() {
	if s.watcher != nil {
		return
	}

	var (
		watcher    cluster.Watcher
		prefixChan <-chan map[string]*string
		err        error
	)

	defer func() {
		if err != nil {
			s.handleWatchFailed()
		}
	}()

	watcher, err = s.cls.Watcher()
	if err != nil {
		logger.Errorf("get watcher failed: %v", err)
		return
	}
	s.watcher = watcher

	prefixChan, err = s.watcher.WatchPrefix(s.prefix)
	if err != nil {
		logger.Errorf("watch prefix failed: %v", err)
		return
	}
	s.prefixChan = prefixChan
}

func (s *Storage) closeWatcher() {
	s.handleWatchFailed()
}

func (s *Storage) handleWatchFailed() {
	if s.watcher != nil {
		s.watcher.Close()
	}
	s.watcher, s.prefixChan = nil, nil
}

func (s *Storage) configToDelta(config map[string]string) map[string]*string {
	delta := make(map[string]*string)
	for k, v := range config {
		k = s.prefix + k
		value := v
		delta[k] = &value
	}

	return delta
}

// loadConfig loads config from the local file.
func (s *Storage) loadConfig() (map[string]string, error) {
	if _, err := os.Stat(s.configfilePath); os.IsNotExist(err) {
		// NOTE: This is not an error.
		logger.Infof("%s not exist", s.configfilePath)
		return nil, nil
	}

	buff, err := ioutil.ReadFile(s.configfilePath)
	if err != nil {
		return nil, fmt.Errorf("%s read failed: %v", s.configfilePath, err)
	}

	config := make(map[string]string)
	err = yaml.Unmarshal(buff, &config)
	if err != nil {
		return nil, fmt.Errorf("%s unmarshal to yaml failed: %v", s.configfilePath, err)
	}

	return config, nil
}

func (s *Storage) storeConfig() {
	buff := bytes.NewBuffer(nil)
	buff.WriteString(fmt.Sprintf("# %s\n", time.Now().Format(time.RFC3339)))

	configBuff, err := yaml.Marshal(s.config)
	if err != nil {
		logger.Errorf("marshal %s to yaml failed: %v", buff, err)
		return
	}
	buff.Write(configBuff)

	err = ioutil.WriteFile(s.configfilePath, buff.Bytes(), 0o644)
	if err != nil {
		logger.Errorf("write %s failed: %v", s.configfilePath, err)
		return
	}
}

func (s *Storage) copyConfig() map[string]string {
	config := make(map[string]string)
	for k, v := range s.config {
		config[k] = v
	}
	return config
}

func (s *Storage) handlSyncStatus(statuses map[string]string) {
	kvs := make(map[string]*string)
	for k, v := range statuses {
		if _, exists := s.config[k]; exists {
			k = s.cls.Layout().StatusObjectKey(k)
			value := v
			kvs[k] = &value
		}
	}

	for k := range s.statusToDelete {
		if _, exists := s.config[k]; !exists {
			k = s.cls.Layout().StatusObjectKey(k)
			kvs[k] = nil
		}
	}

	err := s.cls.PutAndDeleteUnderLease(kvs)
	if err != nil {
		logger.Errorf("sync runtime failed: put status failed: %v", err)
	} else {
		s.statusToDelete = make(map[string]struct{})
	}
}

// Close closes storage.
func (s *Storage) Close() {
	close(s.done)
}
