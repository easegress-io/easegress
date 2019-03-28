package cluster

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/megaease/easegateway/cmd/server/environ"
	"github.com/megaease/easegateway/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.etcd.io/etcd/pkg/types"

	"github.com/megaease/easegateway/pkg/option"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster Test Suite")
}

const (
	NODE_NUM = 5
)

var a = Context("With a 5-node Cluster", func() {

	var (
		clusters    []*cluster
		options     []option.Options
		randomOrder []int
	)

	BeforeEach(func() {
		By("Shuffle the starting order", func() {
			for i := 0; i < NODE_NUM; i++ {
				randomOrder = append(randomOrder, i)
			}

			rand.Shuffle(len(randomOrder), func(i, j int) {
				randomOrder[i], randomOrder[j] = randomOrder[j], randomOrder[i]
			})
		})
	})

	Specify("All nodes should be up", func(done Done) {

		By("Define a test cluster", func() {
			option.Global = option.New()
			clusters, options = defineCluster("cluster-bootstrap", "3")
			logger.Init()
		})

		By("Start all the node in any order ", func() {
			wait := &sync.WaitGroup{}
			wait.Add(NODE_NUM)
			for _, i := range randomOrder {
				var err error
				go func(j int) {
					defer GinkgoRecover()
					var etcdDone chan struct{}
					clusters[j], etcdDone, err = New(options[j])
					<-etcdDone
					Expect(err).To(BeNil(), "Node %d, should start successfully", j)
					wait.Done()
				}(i)
			}
			wait.Wait()

		})

		By("Wait the nodes to learn and save members.yaml", func() {
			time.Sleep(3 * time.Second)
		})

		By("Check the leaderId", func() {
			leaderId := clusters[0].server.Server.Leader()
			id := clusters[0].server.Server.ID()
			Expect(leaderId).To(Equal(id), "Node 1 should be the leader for the bootstrap startup")
		})

		By("Put a value through one node", func() {
			clusters[NODE_NUM-1].client.Put(context.Background(), "/root/egtest/hello", "easegateway")
		})

		By("Get the value from another node", func() {

			resp, err := clusters[NODE_NUM-2].client.Get(context.Background(), "/root/egtest/hello")
			Expect(err).To(BeNil())
			Expect(string(resp.Kvs[0].Value)).To(Equal("easegateway"),
				"data should be retrieved from another node")
		})

		By("Stop the leader node", func() {
			leaderId := clusters[1].server.Server.Leader()
			node0Id := clusters[0].server.Server.ID()
			Expect(leaderId).To(Equal(node0Id), "The bootstrap leader should be node 0")

			clusters[0].server.Close()
			time.Sleep(2 * time.Second)
		})

		By("Confirm a new leader is elected", func() {
			leaderId := clusters[2].server.Server.Leader()
			Expect(leaderId).NotTo(Equal(types.ID(0)), "A new leader node should be elected")
		})

		By("Stop a second node(2 out of 3 are down", func() {
			clusters[1].server.Close()
			time.Sleep(2 * time.Second)
		})

		By("Confirm there's no leader", func() {
			leaderId := clusters[2].server.Server.Leader()
			Expect(leaderId).To(Equal(types.ID(0)), "There should be no leader")
		})

		By("Restart a node", func() {
			var err error
			var etcdDone chan struct{}
			clusters[1], etcdDone, err = New(options[1])
			<-etcdDone
			Expect(err).To(BeNil(), "The node should restart successfully")
			time.Sleep(2 * time.Second)
		})

		By("Confirm a new leader is elected again", func() {
			leaderId := clusters[1].server.Server.Leader()
			Expect(leaderId).NotTo(Equal(types.ID(0)), "A leader should be elected again")
		})

		By("Put another value into the etcd cluster", func() {
			putCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			clusters[NODE_NUM-4].client.Put(putCtx, "/root/egtest/hello", "easegateway 2")
			Expect(putCtx.Err()).To(BeNil())
			cancel()
		})

		By("Get the the same value", func() {
			getCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			resp, err := clusters[NODE_NUM-4].client.Get(getCtx, "/root/egtest/hello")
			Expect(getCtx.Err()).To(BeNil())
			cancel()
			Expect(err).To(BeNil())
			Expect(string(resp.Kvs[0].Value)).To(Equal("easegateway 2"), "Data should be consistent")
		})

		By("Stop all nodes", func() {
			clusters[0].server.Close()
			clusters[1].server.Close()
			clusters[2].server.Close()
			// node 3, 4 has no server
		})

		By("Restart all nodes", func() {
			wait := &sync.WaitGroup{}
			wait.Add(NODE_NUM)
			for _, i := range randomOrder {
				go func(j int) {
					defer GinkgoRecover()
					var err error
					var etcdDone chan struct{}
					clusters[j], etcdDone, err = New(options[j])
					<-etcdDone
					Expect(err).To(BeNil(), "Node %d should start successfully", j)
					wait.Done()
				}(i)
			}
			wait.Wait()
		})

		By("Check the leader id", func() {
			leaderId := clusters[0].server.Server.Leader()
			Expect(leaderId).NotTo(Equal(types.ID(0)))
		})

		By("Put another value into the etcd cluster", func() {
			putCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			clusters[NODE_NUM-4].client.Put(putCtx, "/root/egtest/hello", "easegateway 3")
			Expect(putCtx.Err()).To(BeNil())
			cancel()
		})

		By("Get the the same value", func() {
			getCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			resp, err := clusters[NODE_NUM-4].client.Get(getCtx, "/root/egtest/hello")
			Expect(getCtx.Err()).To(BeNil())
			cancel()
			Expect(err).To(BeNil())
			Expect(string(resp.Kvs[0].Value)).To(Equal("easegateway 3"), "Data should be consistent")
		})

		By("Remove a member", func() {

			m := clusters[1].MemberStatus()

			clusters[1].server.Close()
			time.Sleep(2 * time.Second)

			err := clusters[0].PurgeMember(m.Name)
			Expect(err).To(BeNil())

			removed := clusters[2].server.Server.IsIDRemoved(uint64(m.Id))
			Expect(removed).To(BeTrue(), "Member 1 should be removed")
		})

		close(done)
	}, 30)
})

// Define a 5-node cluster for test. 0,1,2: writer, 3,4: reader.
// The cluster will be deployed under /tmp/<clusterName>
// portChannel is 3-9, the port is portChanel<40><node idx>, eg. 4401
func defineCluster(clusterName string, portChannel string) ([]*cluster, []option.Options) {

	clusters := make([]*cluster, NODE_NUM)
	options := make([]option.Options, NODE_NUM)

	for i := 0; i < NODE_NUM; i++ {
		options[i] = *option.Global

		// verify default Name
		if i%2 == 0 {
			options[i].Name = "egtest-" + strconv.Itoa(i)
		}
		options[i].ClusterName = "cluster-egtest"
		options[i].ClusterClientURL = "http://127.0.0.1:" + portChannel + "400" + strconv.Itoa(i)
		options[i].ClusterPeerURL = "http://127.0.0.1:" + portChannel + "500" + strconv.Itoa(i)
		options[i].APIAddr = "127.0.0.1:" + portChannel + "600" + strconv.Itoa(i)

		// i == 0 means the bootstrap writer, which has no joinUrl
		if i == 1 || i == 2 {
			options[i].ClusterJoinURLs = options[0].ClusterPeerURL
		}

		// readers
		if i >= 3 {
			options[i].ClusterJoinURLs = options[1].ClusterPeerURL + "," + options[1].ClusterPeerURL
		}

		if i >= 3 {
			options[i].ClusterRole = "reader"
		} else {
			options[i].ClusterRole = "writer"
		}

		option.InitConfig(&options[i])

		options[i].DataDir = filepath.Join("/tmp/egtest", clusterName, options[i].Name, "data")
		options[i].LogDir = filepath.Join("/tmp/egtest", clusterName, options[i].Name, "logs")
		options[i].CGIDir = filepath.Join("/tmp/egtest", clusterName, options[i].Name, "cgi")
		options[i].CertDir = filepath.Join("/tmp/egtest", clusterName, options[i].Name, "cert")
		options[i].ConfDir = filepath.Join("/tmp/egtest", clusterName, options[i].Name, "conf")

		// clear all data that may be produce in previous test
		os.RemoveAll(filepath.Join("/tmp/egtest", clusterName, options[i].Name))
		environ.InitDirs(options[i])

	}

	return clusters, options

}
