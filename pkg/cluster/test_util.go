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

	"github.com/phayes/freeport"

	"github.com/megaease/easegress/v2/pkg/env"
	"github.com/megaease/easegress/v2/pkg/option"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// CreateOptionsForTest creates a options for testing purposes.
func CreateOptionsForTest(tempDir string) *option.Options {
	ports, err := freeport.GetFreePorts(3)
	check(err)
	name := "test-member-x"
	opt := option.New()
	opt.Name = name
	opt.ClusterName = "test-cluster"
	opt.ClusterRole = "primary"
	opt.ClusterRequestTimeout = "10s"
	opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", ports[0])}
	opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
	opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", ports[1])}
	opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
	opt.Cluster.InitialCluster = make(map[string]string)
	opt.Cluster.InitialCluster[name] = opt.Cluster.InitialAdvertisePeerURLs[0]
	opt.APIAddr = fmt.Sprintf("localhost:%d", ports[2])
	opt.DataDir = fmt.Sprintf("%s/data", tempDir)
	opt.LogDir = fmt.Sprintf("%s/log", tempDir)

	err = opt.Parse()
	check(err)

	env.InitServerDir(opt)
	return opt
}

// CreateClusterForTest creates a cluster for testing purposes.
func CreateClusterForTest(tempDir string) Cluster {
	opt := CreateOptionsForTest(tempDir)

	clusterInstance, err := New(opt)
	check(err)
	return clusterInstance
}
