package cluster

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/logutils"

	"testutil"
)

var Nodes = 5

// generate high convergence configuration
// we want to gossip aggressive and ignore memberlist logs
func highConvergenceCfg() *Config {
	cfg := testConfig()

	// Set probe intervals that are aggressive to be time efficiency
	cfg.GossipInterval = 5 * time.Millisecond

	// this RetransmitMult is relatively high convergence with 5 cluster Nodes
	// Reference: https: //www.serf.io/docs/internals/simulator.html
	// But becareful the above is just theoritical simulator, so it
	// may differ from real situations
	cfg.MessageRetransmitMult = 7

	// ignore memberlist DEBUG level logs in long run test
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("INFO"),
		Writer:   os.Stderr,
	}

	cfg.LogOutput = filter
	return cfg
}

func testConfig() *Config {
	config := DefaultLANConfig()
	config.BindAddress = testutil.GetBindAddr().String()

	config.GossipInterval = 20 * time.Millisecond
	config.ProbeInterval = 1 * time.Second
	config.ProbeTimeout = 3 * config.ProbeInterval
	config.TCPTimeout = 200 * time.Millisecond

	config.NodeName = fmt.Sprintf("Node %s", config.BindAddress)

	return config

}

// initCluster with specified node numbers
// NOTE: remember to call Cluster.internalStop(true) to leave cluster after test finished
type createConfig func() *Config

func initCluster(Nodes int, initEventChannel bool, cfg createConfig) ([]*Cluster, []chan Event, []*Config, error) {
	cfgs := make([]*Config, Nodes)
	clusters := make([]*Cluster, Nodes)
	eventChs := make([]chan Event, Nodes) // two directional channel
	for i := 0; i < Nodes; i++ {
		if cfg != nil {
			cfgs[i] = cfg()
		} else {
			cfgs[i] = testConfig()
		}
		if initEventChannel {
			eventChs[i] = make(chan Event, 1024)
			cfgs[i].EventStream = eventChs[i] // used as send channel
		}
		var err error
		clusters[i], err = Create(*cfgs[i])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create cluster err: %v", err)
		}

		testutil.Yield()

		retry := 0
		retryLimit := 3
		if i > 0 { // join to (i-1)th node
			for {
				if _, err = clusters[i].Join([]string{cfgs[i-1].BindAddress}); err == nil || retry >= retryLimit {
					break
				}
				retry++
			}
			if err != nil {
				for j := 0; j < i; j++ {
					clusters[j].internalStop(true)
				}
				return nil, nil, nil, fmt.Errorf("%dth node join %dth node failed: %s", i, i-1, err)
			}
			testutil.Yield()
		}
	}
	return clusters, eventChs, cfgs, nil
}

// Test cluster message convergence by send fake request from c0 and record the ack loss
// We use huge experiments to calculate the node unreached possibility under local environment
// convergence possibility = 1 - nodesUnReachedPossibility
func TestCluster_Convergence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cluster convergence test in short mode")
	}
	// checks ack is enough to demonstrate gossip convergence

	// maybe we can make these variables as parameters of `go test` with default values
	queryNum := 10240 // make it larger to calculate more accurate convergence
	maxUnReachedP := 0.0001

	// initCluster without even channel, we only care about acks
	clusters, _, _, err := initCluster(Nodes, false, highConvergenceCfg)
	if err != nil {
		t.Fatalf("Init cluster failed: %v", err)
	}
	for i := range clusters {
		defer clusters[i].internalStop(true)
	}

	c0 := clusters[0]
	params := defaultRequestParam(c0.memberList.NumMembers(), c0.conf.RequestTimeoutMult, c0.conf.GossipInterval)
	t.Logf("Init cluster with %d Nodes successfully:\n"+
		"\tNumMembers: %d\n"+
		"\tGossipRetransmitMult: %d\n"+
		"\tRequestTimeOutMult: %d\n"+
		"\tGossipInterval: %.6fs\n"+
		"\tTimeout: %.6fs",
		Nodes, c0.memberList.NumMembers(), c0.conf.GossipRetransmitMult,
		c0.conf.RequestTimeoutMult, c0.conf.GossipInterval.Seconds(), params.Timeout.Seconds())

	ackLoss := 0
	for i := 0; i < queryNum; i++ {
		requestName := fmt.Sprintf("test request %dth query", i)
		var payload []byte
		// Make a fake query from c0 to the whole cluster members(include self)
		f, err := c0.Request(requestName, payload, params)
		if err != nil {
			t.Fatalf("c0 request %dth query failed, err: %v", i, err)
		}
		ackCh := f.Ack()
		acks := make(map[string]struct{}, Nodes)
		// check acks
		for node := range ackCh {
			if _, ok := acks[node]; ok {
				t.Fatalf("Duplicate ack from node : %s", node)
			}
			acks[node] = struct{}{}
			if len(acks) == Nodes { // enough acks so break
				break
			}
		}
		if len(acks) != Nodes {
			ackLoss++
			t.Logf("missing acks for %dth user query, expected %d, but got %d, %v", i, Nodes, len(acks), acks)
		}
	}
	realUnReachP := float64(ackLoss) / float64(queryNum)
	t.Logf("real unreached possibility: %f", realUnReachP)
	if realUnReachP > maxUnReachedP {
		t.Fatalf("real unreached possibility %.6f > expected max unreached possibility %.6f", realUnReachP, maxUnReachedP)
	}
}

// Test cluster Request by send fake request from c0 and respond this request on every node
// Every Nodes should receive this request, send ack and responses
func TestCluster_BroadcastRequest(t *testing.T) {
	clusters, eventChs, cfgs, err := initCluster(Nodes, true, nil)
	if err != nil {
		t.Fatalf("Init cluster failed: %v", err)
	}
	for i := range clusters {
		defer clusters[i].internalStop(true)
	}

	// nonblocking consumes MemberEvent
	for j := 0; j < Nodes; j++ {
	CONSUME:
		for {
			select {
			case <-eventChs[j]:
			default:
				break CONSUME
			}
		}
	}

	c0 := clusters[0]
	params := defaultRequestParam(c0.memberList.NumMembers(), c0.conf.RequestTimeoutMult, c0.conf.GossipInterval)
	params.Timeout = 10 * time.Second

	requestName := fmt.Sprintf("test request query")
	var payload []byte
	f, err := c0.Request(requestName, payload, params)
	if err != nil {
		t.Fatalf("c0 request query failed, err: %v", err)
	}
	ackCh := f.Ack()
	acks := make(map[string]struct{}, Nodes)
	resps := make(map[string]string, Nodes)

	// check acks
	for node := range ackCh {
		acks[node] = struct{}{}
		if len(acks) == Nodes { // collected enough acks, so break the loop (don't need to wait ackCh closed by timeout)
			break
		}
	}
	if len(acks) != Nodes {
		t.Fatalf("missing acks for request, expected %d, but got %d, %v", Nodes, len(acks), acks)
	}

	// check request events
	// nonblocking receiving events and respond
	// we already recevied acks, so the event should also be ready
	for j := Nodes - 1; j >= 0; j-- {
		select {
		case event := <-eventChs[j]:
			e, ok := event.(*RequestEvent)
			if !ok {
				t.Fatalf("event for request type mismatched, expected: %v, real: %v", reflect.TypeOf(&RequestEvent{}), reflect.TypeOf(event))
			}
			testRequestEvent(t, cfgs[0].NodeName, cfgs[j].NodeName, f, e)
			if err := e.Respond([]byte(fmt.Sprintf("response to request"))); err != nil {
				t.Fatalf("respond to request from %dth node failed: %v", j, err)
			}
		default:
			t.Fatalf("%dth node failed to receive request event timely", j)
		}
	}
	testutil.Yield()
	respCh := f.Response()
	for response := range respCh {
		resps[response.ResponseNodeName] = string(response.Payload)
		if len(resps) == Nodes { // collected enough acks, so break the loop (don't need to wait respCh closed by timeout)
			break
		}
	}

	if len(resps) != Nodes {
		t.Fatalf("missing responses for request, expected %d, but got %d\n\t%v", Nodes, len(resps), resps)
	}
}
