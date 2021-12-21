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
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/option"
)

func TestCreateEtcdConfigFailures(t *testing.T) {
	testData := make([]*option.Options, 0)
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].ClusterListenClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].ClusterListenPeerURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].ClusterAdvertiseClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].ClusterInitialAdvertisePeerURLs = []string{"::::::"}

	for i, opt := range testData {
		t.Run(fmt.Sprintf("CreateEtcdConfig: options invalid url i=%d", i), func(t *testing.T) {
			membersInstance, _ := newMembers(opt)
			_, err := CreateEtcdConfig(opt, membersInstance)
			if err == nil {
				t.Error("There should be an error")
			}
			if !strings.Contains(err.Error(), "missing protocol scheme") {
				t.Error("Error should contain missing protocol scheme")
			}
		})
	}

	testData = make([]*option.Options, 0)
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].Cluster.ListenClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].Cluster.ListenPeerURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
	testData[len(testData)-1].Cluster.AdvertiseClientURLs = []string{"::::::"}
	testData = append(testData, mockTestOpt())
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
