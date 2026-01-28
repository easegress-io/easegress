/*
 * Copyright (c) 2017, The Easegress Authors
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

package metricshub_test

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/cluster/clustertest"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"

	// import this to create spec from supervisor, to avoid import cycle the test file is package metricshub_test
	_ "github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestMetricsHub(t *testing.T) {
	assert := assert.New(t)

	// mock cluster
	clusterData := sync.Map{}
	mockCluster := clustertest.NewMockedCluster()
	mockCluster.MockedLayout = func() *cluster.Layout {
		return &cluster.Layout{}
	}
	mockCluster.MockedPut = func(key string, value string) error {
		clusterData.Store(key, value)
		return nil
	}
	mockCluster.MockedGetPrefix = func(prefix string) (map[string]string, error) {
		res := make(map[string]string)
		clusterData.Range(func(k, v interface{}) bool {
			if strings.HasPrefix(k.(string), prefix) {
				res[k.(string)] = v.(string)
			}
			return true
		})
		return res, nil
	}

	// mock supervisor
	super := supervisor.NewMock(option.New(), mockCluster, nil,
		nil, false, nil, nil)

	controllerConfig := `
kind: AIGatewayController
name: aigatewaycontroller
providers:
- name: openai
  providerType: openai
  baseURL: http://localhost:19876
  apiKey: mock
`
	spec, err := super.NewSpec(controllerConfig)
	assert.Nil(err)

	hub := metricshub.New(spec)
	defer hub.Close()
	for range 10 {
		metric := &metricshub.Metric{
			Success:      true,
			Duration:     100,
			Provider:     "openai",
			ProviderType: "openai",
			InputTokens:  1000,
			OutputTokens: 500,
			Model:        "gpt5",
			BaseURL:      "http://localhost:19876",
			ResponseType: "chat.completions",
		}
		hub.Update(metric)
	}

	stats := hub.GetStats()
	assert.Equal(1, len(stats))
	stat := stats[0]
	assert.Equal("openai", stat.Provider)
	assert.Equal("openai", stat.ProviderType)
	assert.Equal("gpt5", stat.Model)
	assert.Equal("http://localhost:19876", stat.BaseURL)
	assert.Equal("chat.completions", stat.RespType)
	assert.Equal(int64(10), stat.TotalRequests)
	assert.Equal(int64(10), stat.SuccessRequests)
	assert.Equal(int64(0), stat.FailedRequests)
	assert.Equal(int64(10000), stat.PromptTokens)
	assert.Equal(int64(5000), stat.CompletionTokens)

	time.Sleep(6 * time.Second)

	allStats, err := hub.GetAllStats()
	assert.Nil(err)
	assert.Equal(1, len(allStats))
	allStat := allStats[0]
	assert.Equal("openai", allStat.Provider)
	assert.Equal("openai", allStat.ProviderType)
	assert.Equal("gpt5", allStat.Model)
	assert.Equal("http://localhost:19876", allStat.BaseURL)
	assert.Equal("chat.completions", allStat.RespType)
	assert.Equal(int64(10), allStat.TotalRequests)
	assert.Equal(int64(10), allStat.SuccessRequests)
	assert.Equal(int64(0), allStat.FailedRequests)
	assert.Equal(int64(10000), allStat.PromptTokens)
	assert.Equal(int64(5000), allStat.CompletionTokens)
}
