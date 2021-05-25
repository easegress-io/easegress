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

package cluster

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func mockClusters(count int) []*cluster {
	opts, _, _ := mockMembers(count)
	clusters := make([]*cluster, count)

	bootCluster, err := New(opts[0])
	if err != nil {
		panic(fmt.Errorf("new cluster failed: %v", err))
	}
	clusters[0] = bootCluster.(*cluster)
	time.Sleep(HeartbeatInterval)

	for i := 1; i < count; i++ {
		opts[i].ClusterJoinURLs = []string{opts[0].ClusterPeerURL}
		cls, err := New(opts[i])
		if err != nil {
			panic(fmt.Errorf("new cluster failed: %v", err))
		}

		c := cls.(*cluster)

		for {
			err = c.getReady()
			time.Sleep(HeartbeatInterval)
			if err != nil {
				continue
			} else {
				break
			}
		}

		clusters[i] = c
	}

	return clusters
}

func closeClusters(clusters []*cluster) {
	wg := &sync.WaitGroup{}
	wg.Add(len(clusters))

	for _, cls := range clusters {
		cls.Close(wg)
	}
}

func TestCluster(t *testing.T) {
	clusters := mockClusters(5)
	defer closeClusters(clusters)
}
