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
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/phayes/freeport"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/megaease/easegress/v2/pkg/env"
	"github.com/megaease/easegress/v2/pkg/option"
)

var memberCounter = 0

func mockTestOpt(ports []int) *option.Options {
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

	if err := opt.Parse(); err != nil {
		panic(fmt.Errorf("parse option failed: %v", err))
	}

	return opt
}

func mockMembers(count int) ([]*option.Options, membersSlice, []*pb.Member) {
	ports, err := freeport.GetFreePorts(count * 3)
	if err != nil {
		panic(fmt.Errorf("get %d free ports failed: %v", count*3, err))
	}

	opts := make([]*option.Options, count)
	members := make(membersSlice, count)
	pbMembers := make([]*pb.Member, count)

	for i := 0; i < count; i++ {
		id := i + 1
		opt := mockTestOpt(ports[i*3 : (i+1)*3])

		opts[i] = opt
		members[i] = &member{
			ID:      uint64(id),
			Name:    opt.Name,
			PeerURL: opt.Cluster.InitialAdvertisePeerURLs[0],
		}
		pbMembers[i] = &pb.Member{
			ID:         uint64(id),
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
