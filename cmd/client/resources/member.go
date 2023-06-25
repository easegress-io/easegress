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

package resources

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

const MemberName = "member"

func MemberAlias() []string {
	return []string{"m", "mem", "members"}
}

func memberCmd(cmdType general.CmdType) []*cobra.Command {
	switch cmdType {
	case general.GetCmd:
		return memberGetCmd()
	case general.DescribeCmd:
		return memberDescribeCmd()
	case general.DeleteCmd:
		return memberDeleteCmd()
	default:
		return nil
	}
}

func memberDescribeCmd() []*cobra.Command {
	examples := []general.Example{
		{Desc: "Describe all members", Command: "egctl describe member"},
		{Desc: "Describe one member", Command: "egctl describe member <member-name>"},
	}

	cmd := &cobra.Command{
		Use:     MemberName,
		Short:   "Describe one or many members",
		Aliases: MemberAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("requires at most one arg for member name")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makeURL(general.MembersURL), nil, cmd)
			if err != nil {
				general.ExitWithErrorf("get members failed: %v", err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			members := []*cluster.MemberStatus{}
			err = codectool.Unmarshal(body, &members)
			if err != nil {
				general.ExitWithErrorf("get members failed: %v", err)
			}
			if len(args) == 0 {
				printMemberStatusDescription(members)
				return
			}

			for _, member := range members {
				if member.Options.Name == args[0] {
					printMemberStatusDescription([]*cluster.MemberStatus{member})
					return
				}
			}
			general.ExitWithErrorf("member %s not found", args[0])
		},
	}
	return []*cobra.Command{cmd}
}

func printMemberStatusDescription(memberStatus []*cluster.MemberStatus) {
	statusToMapInterface := func(status *cluster.MemberStatus) map[string]interface{} {
		result := map[string]interface{}{}
		result["etcd"] = status.Etcd
		result["lastHeartbeatTime"] = status.LastHeartbeatTime
		result["lastDefragTime"] = status.LastDefragTime

		options := reflect.ValueOf(status.Options)
		for i := 0; i < options.NumField(); i++ {
			if options.Field(i).CanInterface() {
				key := options.Type().Field(i).Name
				value := options.Field(i).Interface()
				result[strings.ToLower(key)] = value
			}
		}
		return result
	}

	results := []map[string]interface{}{}
	for _, status := range memberStatus {
		results = append(results, statusToMapInterface(status))
	}
	general.PrintMapInterface(results, []string{
		"name", "lastHeartbeatTime", "", "etcd", "",
	})
}

func memberDeleteCmd() []*cobra.Command {
	cmd := &cobra.Command{
		Use:     MemberName,
		Aliases: MemberAlias(),
		Short:   "Purge a Easegress member. This command should be run after the easegress node uninstalled",
		Example: createExample("Purge a Easegress member. This command should be run after the easegress node uninstalled", "egctl delete member <member-name>"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one member name to be deleted")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_, err := handleReq(http.MethodDelete, makeURL(general.MemberItemURL, args[0]), nil, cmd)
			if err != nil {
				general.ExitWithErrorf("purge member failed: %v", err)
			}
			fmt.Printf("purge member %s successfully\n", args[0])
		},
	}
	return []*cobra.Command{cmd}
}

func memberGetCmd() []*cobra.Command {
	examples := []general.Example{
		{Desc: "Get all members", Command: "egctl get member"},
		{Desc: "Get one member", Command: "egctl get member <member-name>"},
	}

	cmd := &cobra.Command{
		Use:     MemberName,
		Short:   "Display one or many members",
		Aliases: MemberAlias(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("requires at most one arg for member name")
			}
			return nil
		},
		Example: createMultiExample(examples),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := handleReq(http.MethodGet, makeURL(general.MembersURL), nil, cmd)
			if err != nil {
				general.ExitWithErrorf("get members failed: %v", err)
			}

			if !general.CmdGlobalFlags.DefaultFormat() {
				general.PrintBody(body)
				return
			}

			members := []*cluster.MemberStatus{}
			err = codectool.Unmarshal(body, &members)
			if err != nil {
				general.ExitWithErrorf("get members failed: %v", err)
			}
			if len(args) == 0 {
				printMemberStatusTable(members)
				return
			}

			for _, member := range members {
				if member.Options.Name == args[0] {
					printMemberStatusTable([]*cluster.MemberStatus{member})
					return
				}
			}
			general.ExitWithErrorf("member %s not found", args[0])
		},
	}
	return []*cobra.Command{cmd}
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
