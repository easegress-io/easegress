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
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/phayes/freeport"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/megaease/easegress/pkg/env"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
)

var (
	memberCounter = 0
	tempDir       = os.TempDir()
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func getRandomString(n int) string {
	randBytes := make([]byte, n/2)
	rand.Read(randBytes)
	return fmt.Sprintf("%x", randBytes)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	// logger.InitMock()
	tempDir = path.Join(tempDir, getRandomString(6))
	code := m.Run()
	os.Exit(code)
}

func mockTestOpt() *option.Options {
	ports, err := freeport.GetFreePorts(3)
	if err != nil {
		panic(fmt.Errorf("get 3 free ports failed: %v", err))
	}

	memberCounter++
	name := fmt.Sprintf("test-member-%03d", memberCounter)

	opt := option.New()
	opt.Name = name
	opt.ClusterName = "test-cluster"
	opt.ClusterRole = "primary"
	opt.ClusterRequestTimeout = "10s"
	opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", ports[0])}
	opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
	opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", ports[1])}
	opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
	opt.APIAddr = fmt.Sprintf("localhost:%d", ports[2])
	opt.HomeDir = filepath.Join(tempDir, name)
	opt.DataDir = "data"
	opt.LogDir = "log"
	opt.MemberDir = "member"
	opt.Debug = false

	_, err = opt.Parse()
	if err != nil {
		panic(fmt.Errorf("parse option failed: %v", err))
	}

	return opt
}

func mockMembers(count int) ([]*option.Options, membersSlice, []*pb.Member) {
	opts := make([]*option.Options, count)
	members := make(membersSlice, count)
	pbMembers := make([]*pb.Member, count)

	for i := 0; i < count; i++ {
		opt := mockTestOpt()

		id := uint64(i + 1)

		opts[i] = opt
		members[i] = &member{
			ID:      id,
			Name:    opt.Name,
			PeerURL: opt.Cluster.InitialAdvertisePeerURLs[0],
		}
		pbMembers[i] = &pb.Member{
			ID:         id,
			Name:       opt.Name,
			PeerURLs:   []string{opt.Cluster.InitialAdvertisePeerURLs[0]},
			ClientURLs: []string{opt.Cluster.AdvertiseClientURLs[0]},
		}

		env.InitServerDir(opt)
	}

	initCluster := map[string]string{}
	for _, opt := range opts {
		if opt.ClusterRole == "primary" {
			initCluster[opt.Name] = opt.Cluster.ListenPeerURLs[0]
		}
	}
	for _, opt := range opts {
		if opt.ClusterRole == "primary" {
			opt.Cluster.InitialCluster = initCluster
		}
	}

	sort.Sort(members)

	tmp := members.copy()
	if len(tmp) == 0 {
		panic("members copy failed")
	}
	noexistMember := members.getByPeerURL("no-exist")

	if noexistMember != nil {
		panic("get a member not exist succ, should failed")
	}

	members.deleteByName("no-exist")
	members.deleteByPeerURL("no-exist-purl")
	return opts, members, pbMembers
}

func mockStaticClusterMembers(count int) ([]*option.Options, membersSlice, []*pb.Member) {
	opts := make([]*option.Options, count)
	members := make(membersSlice, count)
	pbMembers := make([]*pb.Member, count)

	portCount := (count * 2) + 1 // two for each member and one for egctl API.
	ports, err := freeport.GetFreePorts(portCount)
	if err != nil {
		panic(fmt.Errorf("get %d free ports failed: %v", portCount, err))
	}
	initialCluster := make(map[string]string)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("static-cluster-test-member-%03d", i)
		peerURL := fmt.Sprintf("http://localhost:%d", ports[(i*2)+1])
		initialCluster[name] = peerURL
	}

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("static-cluster-test-member-%03d", i)
		opt := option.New()
		opt.Name = name
		opt.ClusterName = "test-static-sized-cluster"
		opt.ClusterRole = "primary"
		opt.ClusterRequestTimeout = "10s"
		listenPort := ports[(i*2)+2]
		advertisePort := ports[(i*2)+1]

		opt.APIAddr = fmt.Sprintf("localhost:%d", ports[0])
		opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", listenPort)}
		opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
		opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", advertisePort)}
		opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
		opt.Cluster.InitialCluster = initialCluster
		opt.HomeDir = filepath.Join(tempDir, name)
		opt.DataDir = "data"
		opt.LogDir = "log"
		opt.MemberDir = "member"
		opt.Debug = false
		_, err = opt.Parse() // create directories
		if err != nil {
			panic(fmt.Errorf("parse option failed: %v", err))
		}

		id := uint64(i + 1)

		opts[i] = opt
		members[i] = &member{
			ID:      id,
			Name:    opt.Name,
			PeerURL: opt.Cluster.InitialAdvertisePeerURLs[0],
		}
		pbMembers[i] = &pb.Member{
			ID:         id,
			Name:       opt.Name,
			PeerURLs:   []string{opt.Cluster.InitialAdvertisePeerURLs[0]},
			ClientURLs: []string{opt.Cluster.AdvertiseClientURLs[0]},
		}
		env.InitServerDir(opts[i])
	}
	sort.Sort(members)
	tmp := members.copy()
	if len(tmp) == 0 {
		panic("members copy failed")
	}
	noexistMember := members.getByPeerURL("no-exist")
	if noexistMember != nil {
		panic("get a member not exist succ, should failed")
	}
	members.deleteByName("no-exist")
	members.deleteByPeerURL("no-exist-purl")
	return opts, members, pbMembers
}

func TestUpdateClusterMembers(t *testing.T) {
	opts, ms, pbMembers := mockMembers(9)

	newTestMembers := func() *members {
		m, err := newMembers(opts[0])
		if err != nil {
			panic(fmt.Errorf("new memebrs failed: %v", err))
		}
		return m
	}

	tests := []struct {
		name  string
		want  membersSlice
		input []*pb.Member
	}{
		{
			name:  "1 member",
			want:  membersSlice{ms[0]},
			input: pbMembers[0:1],
		},
		{
			name:  "5 member",
			want:  ms[0:5],
			input: pbMembers[0:5],
		},
		{
			name:  "9 member",
			want:  ms[0:9],
			input: pbMembers[0:9],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMembers()

			m.updateClusterMembers(tt.input)
			got := m.ClusterMembers
			fmt.Printf("got : %+v\n", *got)
			fmt.Printf("want: %+v\n", tt.want)

			if !reflect.DeepEqual(*got, tt.want) {
				t.Fatalf("ClusterMembers want %v, got %v", tt.want, got)
			}
		})
	}
}
