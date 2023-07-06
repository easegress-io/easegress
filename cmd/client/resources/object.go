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

// Package resources provides the resources utilities for the client.
package resources

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/megaease/easegress/cmd/client/general"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// ObjectApiResources returns the object api resources.
func ObjectApiResources() ([]*api.APIResource, error) {
	url := makePath(general.ObjectAPIResources)
	body, err := handleReq(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res := []*api.APIResource{}
	err = codectool.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func defaultObjectNameSpace() string {
	return cluster.TrafficNamespace(cluster.NamespaceDefault)
}

func httpGetObject(name string) ([]byte, error) {
	url := func(name string) string {
		if len(name) == 0 {
			return makePath(general.ObjectsURL)
		}
		return makePath(general.ObjectItemURL, name)
	}(name)
	return handleReq(http.MethodGet, url, nil)
}

// GetAllObject gets all objects.
func GetAllObject(cmd *cobra.Command) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, "resource")
	}

	body, err := httpGetObject("")
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	metas, err := unmarshalMetaSpec(body, true)
	if err != nil {
		return getErr(err)
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Name < metas[j].Name
	})
	printMetaSpec(metas)
	return nil
}

// GetObject gets an object.
func GetObject(cmd *cobra.Command, args *general.ArgInfo, kind string) error {
	msg := fmt.Sprintf("all %s", kind)
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetObject(args.Name)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		if args.ContainName() {
			general.PrintBody(body)
			return nil
		}

		maps, err := general.UnmarshalMapInterface(body, true)
		if err != nil {
			return getErr(err)
		}
		maps = general.Filter(maps, func(m map[string]interface{}) bool {
			return m["kind"] == kind
		})
		newBody, err := codectool.MarshalJSON(maps)
		if err != nil {
			return getErr(err)
		}
		general.PrintBody(newBody)
		return nil
	}

	metas, err := unmarshalMetaSpec(body, !args.ContainName())
	if err != nil {
		return getErr(err)
	}
	metas = general.Filter(metas, func(m *supervisor.MetaSpec) bool {
		return m.Kind == kind
	})

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Name < metas[j].Name
	})
	printMetaSpec(metas)
	return nil
}

func unmarshalMetaSpec(body []byte, listBody bool) ([]*supervisor.MetaSpec, error) {
	if listBody {
		metas := []*supervisor.MetaSpec{}
		err := codectool.Unmarshal(body, &metas)
		return metas, err
	}
	meta := &supervisor.MetaSpec{}
	err := codectool.Unmarshal(body, meta)
	return []*supervisor.MetaSpec{meta}, err
}

func getAgeFromMetaSpec(meta *supervisor.MetaSpec) string {
	createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
	if err != nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(createdAt))
}

func printMetaSpec(metas []*supervisor.MetaSpec) {
	// Output:
	// NAME  KIND  AGE
	// ...
	table := [][]string{}
	table = append(table, []string{"NAME", "KIND", "AGE"})
	for _, meta := range metas {
		table = append(table, []string{meta.Name, meta.Kind, getAgeFromMetaSpec(meta)})
	}
	general.PrintTable(table)
}

// DescribeObject describes an object.
func DescribeObject(cmd *cobra.Command, args *general.ArgInfo, kind string) error {
	msg := fmt.Sprintf("all %s", kind)
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetObject(args.Name)
	if err != nil {
		return getErr(err)
	}

	specs, err := general.UnmarshalMapInterface(body, !args.ContainName())
	if err != nil {
		return getErr(err)
	}

	specs = general.Filter(specs, func(m map[string]interface{}) bool {
		return m["kind"] == kind
	})

	err = addObjectStatusToSpec(specs, args)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		body, err = codectool.MarshalJSON(specs)
		if err != nil {
			return getErr(err)
		}
		general.PrintBody(body)
		return nil
	}

	specials := []string{"name", "kind", "version", "", "flow", "", "filters", "", "rules", ""}
	// Ouput:
	// Name: pipeline-demo
	// Kind: Pipeline
	// Version: xxx
	//
	// Flow:
	// =====
	//   ...
	//
	// Filters:
	// ========
	//   ...
	//
	// Rules:
	// ======
	//   ...
	//
	// ... (other fields)
	//
	// AllStatus:
	// ==========
	//  - node: eg1
	//    status: ...
	//  - node: eg2
	//    status: ...
	general.PrintMapInterface(specs, specials, []string{objectStatusKeyInSpec})
	return nil
}

// CreateObject creates an object.
func CreateObject(cmd *cobra.Command, s *general.Spec) error {
	_, err := handleReq(http.MethodPost, makePath(general.ObjectsURL), []byte(s.Doc()))
	if err != nil {
		return general.ErrorMsg(general.CreateCmd, err, s.Kind, s.Name)
	}
	fmt.Println(general.SuccessMsg(general.CreateCmd, s.Kind, s.Name))
	return nil
}

// DeleteObject deletes an object.
func DeleteObject(cmd *cobra.Command, kind string, names []string, all bool) error {
	// define error msg
	msg := fmt.Sprintf("all %s", kind)
	if !all {
		msg = fmt.Sprintf("%s %s", kind, strings.Join(names, ", "))
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.DeleteCmd, err, msg)
	}

	// get all objects and filter by kind
	body, err := httpGetObject("")
	if err != nil {
		return getErr(err)
	}
	metas, err := unmarshalMetaSpec(body, true)
	if err != nil {
		return getErr(err)
	}
	metas = general.Filter(metas, func(m *supervisor.MetaSpec) bool {
		return m.Kind == kind
	})

	if all {
		for _, m := range metas {
			_, err = handleReq(http.MethodDelete, makePath(general.ObjectItemURL, m.Name), nil)
			if err != nil {
				return getErr(err)
			}
			fmt.Println(general.SuccessMsg(general.DeleteCmd, m.Kind, m.Name))
		}
		return nil
	}
	nameInKind := map[string]struct{}{}
	for _, m := range metas {
		nameInKind[m.Name] = struct{}{}
	}
	for _, name := range names {
		_, ok := nameInKind[name]
		if !ok {
			return getErr(fmt.Errorf("no such %s %s", kind, name))
		}
		_, err = handleReq(http.MethodDelete, makePath(general.ObjectItemURL, name), nil)
		if err != nil {
			return getErr(err)
		}
		fmt.Println(general.SuccessMsg(general.DeleteCmd, kind, name))
	}
	return nil
}

// ApplyObject applies an object.
func ApplyObject(cmd *cobra.Command, s *general.Spec) error {
	checkObjExist := func(cmd *cobra.Command, name string) bool {
		_, err := httpGetObject(name)
		return err == nil
	}

	createOrUpdate := func(cmd *cobra.Command, s *general.Spec, exist bool) error {
		if exist {
			_, err := handleReq(http.MethodPut, makePath(general.ObjectItemURL, s.Name), []byte(s.Doc()))
			return err
		}
		_, err := handleReq(http.MethodPost, makePath(general.ObjectsURL), []byte(s.Doc()))
		return err
	}

	exist := checkObjExist(cmd, s.Name)
	action := general.CreateCmd
	if exist {
		action = "update"
	}
	err := createOrUpdate(cmd, s, exist)
	if err != nil {
		return general.ErrorMsg(action, err, s.Kind, s.Name)
	}
	fmt.Println(general.SuccessMsg(action, s.Kind, s.Name))
	return nil
}

type objectStatusInfo struct {
	namespace string
	name      string
	node      string
	spec      map[string]interface{}
	status    map[string]interface{}
}

func splitObjectStatusKey(key string) (*objectStatusInfo, error) {
	s := strings.Split(key, "/")
	if len(s) != 3 {
		return nil, errors.New("invalid status key")
	}
	return &objectStatusInfo{
		namespace: s[0],
		name:      s[1],
		node:      s[2],
	}, nil
}

// ObjectStatus is the status of an object.
type ObjectStatus struct {
	Spec   map[string]interface{} `json:"spec"`
	Status map[string]interface{} `json:"status"`
}

func unmarshalObjectStatus(data []byte) (ObjectStatus, error) {
	var status ObjectStatus
	err := codectool.Unmarshal(data, &status)
	return status, err
}

func unmarshalObjectStatusInfo(body []byte, name string) ([]*objectStatusInfo, error) {
	kvs := map[string]interface{}{}
	err := codectool.Unmarshal(body, &kvs)
	if err != nil {
		return nil, err
	}

	res := []*objectStatusInfo{}
	for k, v := range kvs {
		info, err := splitObjectStatusKey(k)
		if err != nil {
			return nil, err
		}
		// only show objects in default namespace, objects in other namespaces is not created by user.
		if info.namespace != defaultObjectNameSpace() {
			continue
		}
		if name != "" && info.name != name {
			continue
		}

		statusByte, err := codectool.MarshalJSON(v)
		if err != nil {
			return nil, err
		}
		status, err := unmarshalObjectStatus(statusByte)
		if err != nil {
			return nil, err
		}
		info.spec = status.Spec
		info.status = status.Status
		res = append(res, info)
	}
	return res, nil
}

// NodeStatus is the status of a node.
type NodeStatus struct {
	Node   string
	Status map[string]interface{}
}

const objectStatusKeyInSpec = "allStatus"

func addObjectStatusToSpec(specs []map[string]interface{}, args *general.ArgInfo) error {
	getUrl := func(args *general.ArgInfo) string {
		if !args.ContainName() {
			return makePath(general.StatusObjectsURL)
		}
		return makePath(general.StatusObjectItemURL, args.Name)
	}

	body, err := handleReq(http.MethodGet, getUrl(args), nil)
	if err != nil {
		return err
	}

	infos, err := unmarshalObjectStatusInfo(body, args.Name)
	if err != nil {
		return err
	}

	// key is name, value is array of node and status
	status := map[string][]*NodeStatus{}
	for _, info := range infos {
		status[info.name] = append(status[info.name], &NodeStatus{
			Node:   info.node,
			Status: info.status,
		})
	}

	for _, s := range specs {
		name := s["name"].(string)
		s[objectStatusKeyInSpec] = status[name]
	}
	return nil
}
