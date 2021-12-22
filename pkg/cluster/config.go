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
	"github.com/megaease/easegress/pkg/option"
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

	// Should help to decrease memory usage.
	// Reference: https://etcd.io/docs/v3.5/tuning/#snapshot-tuning
	snapshotCount = 5000
)

var (
	autoCompactionRetention = "10"
	autoCompactionMode      = embed.CompactorModeRevision
)

// CreateStaticClusterEtcdConfig creates an embedded etcd config for static sized cluster,
// listing all cluster members for etcd's initial-cluster argument.
func CreateStaticClusterEtcdConfig(opt *option.Options) (*embed.Config, error) {
	ec := embed.NewConfig()

	var (
		clientURLs   []url.URL
		peerURLs     []url.URL
		clientAdURLs []url.URL
		peerAdURLs   []url.URL
	)
	clientURLs, err := option.ParseURLs(opt.Cluster.ListenClientURLs)
	if err != nil {
		return nil, err
	}
	peerURLs, err = option.ParseURLs(opt.Cluster.ListenPeerURLs)
	if err != nil {
		return nil, err
	}
	clientAdURLs, err = option.ParseURLs(opt.Cluster.AdvertiseClientURLs)
	if err != nil {
		return nil, err
	}
	peerAdURLs, err = option.ParseURLs(opt.Cluster.InitialAdvertisePeerURLs)
	if err != nil {
		return nil, err
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
	ec.SnapshotCount = snapshotCount
	ec.Logger = "zap"
	ec.LogOutputs = []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, logFilename))}

	ec.ClusterState = embed.ClusterStateFlagNew
	if opt.Cluster.StateFlag == "existing" {
		ec.ClusterState = embed.ClusterStateFlagExisting
	}
	ec.InitialCluster = opt.InitialClusterToString()

	logger.Infof("etcd config: init-cluster:%s cluster-state:%s force-new-cluster:%v",
		ec.InitialCluster, ec.ClusterState, ec.ForceNewCluster)

	return ec, nil
}

// CreateEtcdConfig creates an embedded etcd config that starts the cluster by adding member by member.
func CreateEtcdConfig(opt *option.Options, members *members) (*embed.Config, error) {
	ec := embed.NewConfig()

	var (
		clientURLs   []url.URL
		peerURLs     []url.URL
		clientAdURLs []url.URL
		peerAdURLs   []url.URL
	)
	clientURLs, err := option.ParseURLs(opt.ClusterListenClientURLs)
	if err != nil {
		return nil, err
	}
	peerURLs, err = option.ParseURLs(opt.ClusterListenPeerURLs)
	if err != nil {
		return nil, err
	}
	clientAdURLs, err = option.ParseURLs(opt.ClusterAdvertiseClientURLs)
	if err != nil {
		return nil, err
	}
	peerAdURLs, err = option.ParseURLs(opt.ClusterInitialAdvertisePeerURLs)
	if err != nil {
		return nil, err
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
	ec.SnapshotCount = snapshotCount
	ec.Logger = "zap"
	ec.LogOutputs = []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, logFilename))}

	ec.ClusterState = embed.ClusterStateFlagExisting
	if opt.ForceNewCluster {
		ec.ClusterState = embed.ClusterStateFlagNew
		ec.ForceNewCluster = true
		self := members.self()
		ec.InitialCluster = fmt.Sprintf("%s=%s", self.Name, self.PeerURL)
	} else {
		if len(opt.ClusterJoinURLs) == 0 {
			if members.clusterMembersLen() == 1 &&
				common.IsDirEmpty(opt.AbsDataDir) {
				ec.ClusterState = embed.ClusterStateFlagNew
			}
		} else if members.clusterMembersLen() == 1 {
			return nil, fmt.Errorf("join mode with only one cluster member: %v",
				*members.ClusterMembers)
		}
		ec.InitialCluster = members.initCluster()
	}

	logger.Infof("etcd config: init-cluster:%s cluster-state:%s force-new-cluster:%v",
		ec.InitialCluster, ec.ClusterState, ec.ForceNewCluster)

	return ec, nil
}
