package cluster

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func mockClusters(count int) []*cluster {
	opts, _, _ := mockMembers(count)
	clusters := make([]*cluster, count)

	bootCluster, err := New(opts[0])
	if err != nil {
		panic(fmt.Errorf("new cluster failed: %v", err))
	}
	clusters[0] = bootCluster.(*cluster)
	time.Sleep(HeartbeatInterval)

	for i := 1; i < count; i++ {
		opts[i].ClusterJoinURLs = opts[0].ClusterPeerURL
		cls, err := New(opts[i])
		if err != nil {
			panic(fmt.Errorf("new cluster failed: %v", err))
		}

		c := cls.(*cluster)

		for {
			err = c.getReady()
			time.Sleep(HeartbeatInterval)
			if err != nil {
				continue
			} else {
				break
			}
		}

		clusters[i] = c
	}

	return clusters
}

func closeClusters(clusters []*cluster) {
	wg := &sync.WaitGroup{}
	wg.Add(len(clusters))

	for _, cls := range clusters {
		cls.Close(wg)
	}
}

func TestCluster(t *testing.T) {
	clusters := mockClusters(5)
	defer closeClusters(clusters)
}
