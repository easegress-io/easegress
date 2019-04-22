package cluster

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/megaease/easegateway/pkg/option"

	"go.etcd.io/etcd/embed"
)

func generateEtcdConfigFromOption(opt *option.Options, initCluster string) (*embed.Config, error) {
	ec := embed.NewConfig()

	peerURL, err := url.Parse(opt.ClusterPeerURL)
	if err != nil {
		return nil, err
	}

	clientURL, err := url.Parse(opt.ClusterClientURL)
	if err != nil {
		return nil, err
	}

	ec.Name = opt.Name

	ec.Dir = opt.DataDir
	ec.WalDir = opt.WALDir
	ec.InitialClusterToken = opt.ClusterName
	ec.EnableV2 = false
	ec.LPUrls = []url.URL{*peerURL}
	ec.APUrls = []url.URL{*peerURL}
	ec.LCUrls = []url.URL{*clientURL}
	ec.ACUrls = []url.URL{*clientURL}
	ec.AutoCompactionMode = "periodic"
	ec.AutoCompactionRetention = "24h"
	// https://github.com/etcd-io/etcd/issues/10364
	ec.Logger = "zap"
	ec.LogOutputs = []string{filepath.Join(opt.LogDir, "etcd.log")}

	if opt.IsBootstrapWriter {
		// Bootstrap a new cluster.
		ec.ClusterState = embed.ClusterStateFlagNew
		ec.InitialCluster = fmt.Sprintf("%s=%s", ec.Name, opt.ClusterPeerURL)
		ec.ForceNewCluster = opt.ForceNewCluster
	} else {
		// Add this member to the existed cluster.
		ec.ClusterState = embed.ClusterStateFlagExisting
		ec.InitialCluster = initCluster
	}

	return ec, nil
}
