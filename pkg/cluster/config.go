package cluster

import (
	"fmt"
	"net/url"

	"github.com/megaease/easegateway/pkg/option"

	"github.com/coreos/etcd/embed"
)

func generateEtcdConfig(initCluster string) *embed.Config {
	ec := embed.NewConfig()

	peerURL, _ := url.Parse(option.Global.ClusterPeerURL)
	clientURL, _ := url.Parse(option.Global.ClusterClientURL)

	ec.Name = option.Global.Name
	ec.Dir = option.Global.DataDir
	ec.WalDir = option.Global.WALDir
	ec.InitialClusterToken = option.Global.ClusterName
	ec.EnableV2 = false
	ec.LPUrls = []url.URL{*peerURL}
	ec.APUrls = []url.URL{*peerURL}
	ec.LCUrls = []url.URL{*clientURL}
	ec.ACUrls = []url.URL{*clientURL}
	ec.AutoCompactionMode = "periodic"
	ec.AutoCompactionRetention = "24h"
	// https://github.com/etcd-io/etcd/issues/10364
	// ec.Logger = "zap"
	// ec.LogOutputs = []string{filepath.Join(option.Global.LogDir, "etcd.log")}

	if option.Global.ClusterJoinURL == "" {
		// Bootstrap a new cluster.
		ec.ClusterState = embed.ClusterStateFlagNew
		ec.InitialCluster = fmt.Sprintf("%s=%s", option.Global.Name, option.Global.ClusterPeerURL)
		ec.ForceNewCluster = option.Global.ForceNewCluster
	} else {
		// Add this member to the existed cluster.
		ec.ClusterState = embed.ClusterStateFlagExisting
		ec.InitialCluster = initCluster
	}

	return ec
}
