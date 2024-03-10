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
	"net/url"
	"path/filepath"

	"go.etcd.io/etcd/server/v3/embed"

	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
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

	// Threshold for number of changes etcd stores in memory before creating a new snapshot.
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
	ec.ListenClientUrls = clientURLs
	ec.AdvertiseClientUrls = clientAdURLs
	ec.ListenPeerUrls = peerURLs
	ec.AdvertisePeerUrls = peerAdURLs
	ec.AutoCompactionMode = autoCompactionMode
	ec.AutoCompactionRetention = autoCompactionRetention
	ec.QuotaBackendBytes = quotaBackendBytes
	ec.MaxTxnOps = maxTxnOps
	ec.MaxRequestBytes = maxRequestBytes
	ec.SnapshotCount = snapshotCount
	ec.Logger = "zap"

	ec.LogOutputs = []string{"stdout"}
	if opt.AbsLogDir != "" {
		ec.LogOutputs = []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, logFilename))}
	}

	ec.ClusterState = embed.ClusterStateFlagNew
	if opt.Cluster.StateFlag == "existing" {
		ec.ClusterState = embed.ClusterStateFlagExisting
	}
	ec.InitialCluster = opt.InitialClusterToString()

	logger.Infof("etcd config: advertise-client-urls: %+v advertise-peer-urls: %+v init-cluster: %s cluster-state: %s force-new-cluster: %v",
		ec.AdvertiseClientUrls, ec.AdvertisePeerUrls,
		ec.InitialCluster, ec.ClusterState, ec.ForceNewCluster)

	return ec, nil
}
