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

package cluster

import (
	"fmt"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/phayes/freeport"
)

func TestCreateEtcdConfigFailures(t *testing.T) {
	ports, err := freeport.GetFreePorts(4 * 3)
	if err != nil {
		panic(fmt.Errorf("get %d free ports failed: %v", 4*3, err))
	}

	testData := make([]*option.Options, 0)
	testData = append(testData, mockTestOpt(ports[0:3]))
	testData[len(testData)-1].Cluster.ListenClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt(ports[3:6]))
	testData[len(testData)-1].Cluster.ListenPeerURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt(ports[6:9]))
	testData[len(testData)-1].Cluster.AdvertiseClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt(ports[9:12]))
	testData[len(testData)-1].Cluster.InitialAdvertisePeerURLs = []string{"::::::"}

	for i, opt := range testData {
		t.Run(fmt.Sprintf("CreateStaticClusterEtcdConfig: options invalid url i=%d", i), func(t *testing.T) {
			_, err := CreateStaticClusterEtcdConfig(opt)
			if err == nil {
				t.Error("There should be an error")
			}
			if !strings.Contains(err.Error(), "missing protocol scheme") {
				t.Error("Error should contain missing protocol scheme")
			}
		})
	}
}
