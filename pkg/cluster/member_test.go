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
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/megaease/easegateway/pkg/env"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	"github.com/phayes/freeport"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
)

const tempDir = "/tmp/eg-test"

var memberCounter = 0

func TestMain(m *testing.M) {
	absLogDir := filepath.Join(tempDir, "global-log")
	os.MkdirAll(absLogDir, 0755)
	logger.Init(&option.Options{
		Name:      "member-for-log",
		AbsLogDir: absLogDir,
	})

	code := m.Run()

	logger.Sync()
	os.RemoveAll(tempDir)

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
	opt.ClusterRole = "writer"
	opt.ClusterRequestTimeout = "10s"
	opt.ClusterClientURL = fmt.Sprintf("http://localhost:%d", ports[0])
	opt.ClusterPeerURL = fmt.Sprintf("http://localhost:%d", ports[1])
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
			PeerURL: opt.ClusterPeerURL,
		}
		pbMembers[i] = &pb.Member{
			ID:         id,
			Name:       opt.Name,
			PeerURLs:   []string{opt.ClusterPeerURL},
			ClientURLs: []string{opt.ClusterClientURL},
		}

		env.InitServerDir(opt)
	}

	sort.Sort(members)

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
