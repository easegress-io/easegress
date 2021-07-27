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
	"net/url"
	"path/filepath"

	"go.etcd.io/etcd/server/v3/embed"

	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/logger"
)

const (
	logFilename = "etcd_server.log"

	// Reference:
	// https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/maintenance.md#space-quota
	// https://github.com/etcd-io/etcd/blob/master/Documentation/dev-guide/limit.md#storage-size-limit
	// 8GB
	quotaBackendBytes = 8 * 1024 * 1024 * 1024

	maxTxnOps       = 10240
	maxRequestBytes = 10 * 1024 * 1024 // 10MB
)

var (
	autoCompactionRetention = "10"
	autoCompactionMode      = embed.CompactorModeRevision
)

func (c *cluster) prepareEtcdConfig() (*embed.Config, error) {
	ec := embed.NewConfig()
	opt := c.opt

	var (
		clientURLs   []url.URL
		peerURLs     []url.URL
		clientAdURLs []url.URL
		peerAdURLs   []url.URL
	)
	for _, u := range opt.ClusterListenClientURLs {
		clientURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		clientURLs = append(clientURLs, *clientURL)
	}
	for _, u := range opt.ClusterListenPeerURLs {
		peerURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		peerURLs = append(peerURLs, *peerURL)
	}
	for _, u := range opt.ClusterAdvertiseClientURLs {
		clientAdURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		clientAdURLs = append(clientAdURLs, *clientAdURL)
	}
	for _, u := range opt.ClusterInitialAdvertisePeerURLs {
		peerAdURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		peerAdURLs = append(peerAdURLs, *peerAdURL)
	}

	ec.Name = opt.Name

	ec.Dir = opt.AbsDataDir
	ec.WalDir = opt.AbsWALDir
	ec.InitialClusterToken = opt.ClusterName
	ec.EnableV2 = false
	ec.LCUrls = clientURLs
	ec.ACUrls = clientAdURLs
	ec.LPUrls = peerURLs
	ec.APUrls = peerAdURLs
	ec.AutoCompactionMode = autoCompactionMode
	ec.AutoCompactionRetention = autoCompactionRetention
	ec.QuotaBackendBytes = quotaBackendBytes
	ec.MaxTxnOps = maxTxnOps
	ec.MaxRequestBytes = maxRequestBytes
	ec.Logger = "zap"
	ec.LogOutputs = []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, logFilename))}

	ec.ClusterState = embed.ClusterStateFlagExisting
	if c.opt.ForceNewCluster {
		ec.ClusterState = embed.ClusterStateFlagNew
		ec.ForceNewCluster = true
		self := c.members.self()
		ec.InitialCluster = fmt.Sprintf("%s=%s", self.Name, self.PeerURL)
	} else {
		if len(c.opt.ClusterJoinURLs) == 0 {
			if c.members.clusterMembersLen() == 1 &&
				common.IsDirEmpty(c.opt.AbsDataDir) {
				ec.ClusterState = embed.ClusterStateFlagNew
			}
		} else if c.members.clusterMembersLen() == 1 {
			return nil, fmt.Errorf("join mode with only one cluster member: %v",
				*c.members.ClusterMembers)
		}
		ec.InitialCluster = c.members.initCluster()
	}

	logger.Infof("etcd config: init-cluster:%s cluster-state:%s force-new-cluster:%v",
		ec.InitialCluster, ec.ClusterState, ec.ForceNewCluster)

	return ec, nil
}
