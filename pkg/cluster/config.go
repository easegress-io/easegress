package cluster

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/megaease/easegateway/pkg/option"

	"go.etcd.io/etcd/embed"
)

func generateEtcdConfigFromOption(opt option.Options, initCluster string, knownMembers *members) (*embed.Config, error) {
	ec := embed.NewConfig()

	peerURL, err := url.Parse(opt.ClusterPeerURL)
	if err != nil {
		return nil, err
	}

	clientURL, err := url.Parse(opt.ClusterClientURL)
	if err != nil {
		return nil, err
	}

	ec.Name, err = generateMemberName(opt.ClusterPeerURL)
	if err != nil {
		return nil, err
	}

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

	if opt.ClusterJoinURL == "" {
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

func generateMemberName(peerUrl string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	peerURL, err := url.Parse(peerUrl)
	if err != nil {
		return "", err
	}

	memberName := hostname + "-" + peerURL.Host
	memberName = strings.Replace(memberName, ",", "-", -1)
	memberName = strings.Replace(memberName, "=", "-", -1)
	return memberName, nil

}
