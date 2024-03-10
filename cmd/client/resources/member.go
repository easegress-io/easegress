/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package resources provides the resources utilities for the client.
package resources

import (
	"fmt"
	"net/http"
	"time"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// MemberKind is the kind of the member.
const MemberKind = "Member"

// Member returns the member resource.
func Member() *api.APIResource {
	return &api.APIResource{
		Kind:    MemberKind,
		Name:    "member",
		Aliases: []string{"m", "mem", "members"},
	}
}

// DescribeMember describes the member.
func DescribeMember(_ *cobra.Command, args *general.ArgInfo) error {
	msg := "all " + MemberKind
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", MemberKind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.DescribeCmd, err, msg)
	}

	body, err := handleReq(http.MethodGet, makePath(general.MembersURL), nil)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	members := []*cluster.MemberStatus{}
	err = codectool.Unmarshal(body, &members)
	if err != nil {
		general.ExitWithErrorf("get members failed: %v", err)
	}
	if !args.ContainName() {
		printMemberStatusDescription(members)
		return nil
	}

	for _, member := range members {
		if member.Options.Name == args.Name {
			printMemberStatusDescription([]*cluster.MemberStatus{member})
			return nil
		}
	}
	return getErr(fmt.Errorf("member %s not found", args.Name))
}

func printMemberStatusDescription(memberStatus []*cluster.MemberStatus) {
	statusToMapInterface := func(status *cluster.MemberStatus) map[string]interface{} {
		result := map[string]interface{}{}
		result["etcd"] = status.Etcd
		result["lastHeartbeatTime"] = status.LastHeartbeatTime
		result["lastDefragTime"] = status.LastDefragTime

		options, err := codectool.MarshalJSON(status.Options)
		if err != nil {
			general.ExitWithErrorf("marshal options failed: %v", err)
		}
		optionMap := map[string]interface{}{}
		err = codectool.Unmarshal(options, &optionMap)
		if err != nil {
			general.ExitWithErrorf("unmarshal options failed: %v", err)
		}
		for k, v := range optionMap {
			result[k] = v
		}
		return result
	}

	results := []map[string]interface{}{}
	for _, status := range memberStatus {
		results = append(results, statusToMapInterface(status))
	}
	general.PrintMapInterface(results, []string{
		"name", "lastHeartbeatTime", "", "etcd", "",
	}, []string{})
}

// DeleteMember deletes the member.
func DeleteMember(_ *cobra.Command, names []string) error {
	for _, name := range names {
		_, err := handleReq(http.MethodDelete, makePath(general.MemberItemURL, name), nil)
		if err != nil {
			return general.ErrorMsg("purge", err, MemberKind, name)
		}
		fmt.Println(general.SuccessMsg("purge", MemberKind, name))
	}
	return nil
}

// GetMember gets the member.
func GetMember(_ *cobra.Command, args *general.ArgInfo) (err error) {
	msg := "all " + MemberKind
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", MemberKind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := handleReq(http.MethodGet, makePath(general.MembersURL), nil)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	members := []*cluster.MemberStatus{}
	err = codectool.Unmarshal(body, &members)
	if err != nil {
		return getErr(err)
	}
	if !args.ContainName() {
		printMemberStatusTable(members)
		return nil
	}

	for _, member := range members {
		if member.Options.Name == args.Name {
			printMemberStatusTable([]*cluster.MemberStatus{member})
			return nil
		}
	}
	return getErr(fmt.Errorf("member %s not found", args.Name))
}

type member struct {
	Name      string
	Role      string
	APIAddr   string
	Heartbeat *time.Time

	State     string
	StartTime *time.Time
}

func (m *member) age() string {
	if m.StartTime == nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(*m.StartTime))
}

func (m *member) heartbeatAge() string {
	if m.Heartbeat == nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(*m.Heartbeat)) + " ago"
}

func newMember(ms *cluster.MemberStatus) *member {
	member := &member{}
	member.Name = ms.Options.Name
	member.Role = ms.Options.ClusterRole
	member.APIAddr = ms.Options.APIAddr

	heartbeat, err := time.Parse(time.RFC3339, ms.LastHeartbeatTime)
	if err == nil {
		member.Heartbeat = &heartbeat
	}

	if ms.Etcd != nil {
		startTime, err := time.Parse(time.RFC3339, ms.Etcd.StartTime)
		if err == nil {
			member.StartTime = &startTime
		}
		member.State = ms.Etcd.State
	}
	return member
}

func printMemberStatusTable(memberStatus []*cluster.MemberStatus) {
	// Output table:
	// NAME  ROLE  AGE  STATE  API-ADDR  HEARTBEAT
	table := [][]string{}
	table = append(table, []string{"NAME", "ROLE", "AGE", "STATE", "API-ADDR", "HEARTBEAT"})
	for _, ms := range memberStatus {
		member := newMember(ms)
		table = append(table, []string{
			member.Name,
			member.Role,
			member.age(),
			member.State,
			member.APIAddr,
			member.heartbeatAge(),
		})
	}
	general.PrintTable(table)
}
