package cluster

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/megaease/easegateway/pkg/common"

	"github.com/megaease/easegateway/pkg/logger"
	"go.etcd.io/etcd/embed"
)

func (c *cluster) prepareEtcdConfig() (*embed.Config, error) {
	ec := embed.NewConfig()
	opt := c.opt

	peerURL, err := url.Parse(opt.ClusterPeerURL)
	if err != nil {
		return nil, err
	}

	clientURL, err := url.Parse(opt.ClusterClientURL)
	if err != nil {
		return nil, err
	}

	ec.Name = opt.Name

	ec.Dir = opt.AbsDataDir
	ec.WalDir = opt.AbsWALDir
	ec.InitialClusterToken = opt.ClusterName
	ec.EnableV2 = false
	ec.LPUrls = []url.URL{*peerURL}
	ec.APUrls = []url.URL{*peerURL}
	ec.LCUrls = []url.URL{*clientURL}
	ec.ACUrls = []url.URL{*clientURL}
	ec.AutoCompactionMode = "periodic"
	ec.AutoCompactionRetention = "24h"
	// https://github.com/etcd-io/etcd/issues/10364
	// FIXME: Upgrade all etcd package after it jumps into v3.4.
	// Reference: https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/configuration.md#logging-flags
	ec.Logger = "zap"
	ec.LogOutputs = []string{filepath.Join(opt.AbsLogDir, "etcd.log")}

	ec.ClusterState = embed.ClusterStateFlagExisting
	if c.opt.ForceNewCluster {
		ec.ClusterState = embed.ClusterStateFlagNew
		ec.ForceNewCluster = true
		self := c.members.self()
		ec.InitialCluster = fmt.Sprintf("%s=%s", self.Name, self.PeerURL)
	} else {
		if c.opt.ClusterJoinURLs == "" {
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
