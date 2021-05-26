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

package api

import (
	"fmt"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegress/pkg/cluster"
	yaml "gopkg.in/yaml.v2"
)

func (s *Server) setupMemberAPIs() {
	memberAPIs := []*APIEntry{
		{
			Path:    "/status/members",
			Method:  "GET",
			Handler: s.listMembers,
		},
		{
			Path:    "/status/members/{member:string}",
			Method:  "DELETE",
			Handler: s.purgeMember,
		},
	}

	s.RegisterAPIs(memberAPIs)
}

type (
	// ListMembersResp is the response of list member.
	ListMembersResp []cluster.MemberStatus
)

func (r ListMembersResp) Len() int           { return len(r) }
func (r ListMembersResp) Less(i, j int) bool { return r[i].Options.Name < r[j].Options.Name }
func (r ListMembersResp) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

// These methods which operate with cluster guarantee atomicity.

func (s *Server) listMembers(ctx iris.Context) {
	kv, err := s.cluster.GetPrefix(s.cluster.Layout().StatusMemberPrefix())
	if err != nil {
		ClusterPanic(err)
	}

	resp := make(ListMembersResp, 0)
	for _, v := range kv {
		memberStatus := cluster.MemberStatus{}
		err := yaml.Unmarshal([]byte(v), &memberStatus)
		if err != nil {
			panic(fmt.Errorf("unmarshal %s to member status failed: %v", v, err))
		}

		resp = append(resp, memberStatus)
	}

	sort.Sort(resp)

	buff, err := yaml.Marshal(resp)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", resp, err))
	}

	ctx.Write(buff)
}

func (s *Server) purgeMember(ctx iris.Context) {
	memberName := ctx.Params().Get("member")

	s.Lock()
	defer s.Unlock()

	leaseStr, err := s.cluster.Get(s.cluster.Layout().OtherLease(memberName))
	if err != nil {
		ClusterPanic(err)
	}
	if leaseStr == nil {
		HandleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	s._purgeMember(memberName)
}
