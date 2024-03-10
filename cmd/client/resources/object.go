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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// ObjectNamespaceFlags contains the flags for get objects.
type ObjectNamespaceFlags struct {
	Namespace    string
	AllNamespace bool
}

var globalAPIResources []*api.APIResource

// ObjectAPIResources returns the object api resources.
func ObjectAPIResources() ([]*api.APIResource, error) {
	if globalAPIResources != nil {
		return globalAPIResources, nil
	}
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
	globalAPIResources = res
	return res, nil
}

// trafficObjectStatusNamespace return namespace of traffic object status.
// In easegress, when we put traffic object like httpserver, pipeline into namespace "default",
// then their status will be put into namespace "eg-traffic-default".
// the status is updated by TrafficController.
func trafficObjectStatusNamespace(objectNamespace string) string {
	return cluster.TrafficNamespace(objectNamespace)
}

func httpGetObject(name string, flags *ObjectNamespaceFlags) ([]byte, error) {
	objectURL := func(name string) string {
		if len(name) == 0 {
			return makePath(general.ObjectsURL)
		}
		return makePath(general.ObjectItemURL, name)
	}(name)
	if flags != nil {
		values := url.Values{}
		values.Add("namespace", flags.Namespace)
		values.Add("all-namespaces", strconv.FormatBool(flags.AllNamespace))
		objectURL = fmt.Sprintf("%s?%s", objectURL, values.Encode())
	}
	return handleReq(http.MethodGet, objectURL, nil)
}

// GetAllObject gets all objects.
func GetAllObject(cmd *cobra.Command, flags *ObjectNamespaceFlags) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, "resource")
	}

	body, err := httpGetObject("", flags)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		general.PrintBody(body)
		return nil
	}

	if flags.AllNamespace {
		err := unmarshalPrintNamespaceMetaSpec(body, nil)
		if err != nil {
			return getErr(err)
		}
		return nil
	}

	err = unmarshalPrintMetaSpec(body, true, nil)
	if err != nil {
		return getErr(err)
	}
	return nil
}

// EditObject edit an object.
func EditObject(cmd *cobra.Command, args *general.ArgInfo, kind string) error {
	getErr := func(err error) error {
		return general.ErrorMsg(general.EditCmd, err, kind, args.Name)
	}

	// get old yaml and save it to a temp file
	var oldYaml string
	var err error
	if oldYaml, err = getObjectYaml(args.Name); err != nil {
		return getErr(err)
	}
	filePath := getResourceTempFilePath(kind, args.Name)
	newYaml, err := editResource(oldYaml, filePath)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	if newYaml == "" {
		return nil
	}

	_, newSpec, err := general.CompareYamlNameKind(oldYaml, newYaml)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	err = ApplyObject(cmd, newSpec)
	if err != nil {
		return getErr(editErrWithPath(err, filePath))
	}
	os.Remove(filePath)
	return nil
}

func getObjectYaml(objectName string) (string, error) {
	body, err := httpGetObject(objectName, nil)
	if err != nil {
		return "", err
	}

	yamlBody, err := codectool.JSONToYAML(body)
	if err != nil {
		return "", err
	}

	// reorder yaml, put name, kind in front of other fields.
	lines := strings.Split(string(yamlBody), "\n")
	var name, kind string
	var sb strings.Builder
	sb.Grow(len(yamlBody))
	for _, l := range lines {
		if strings.HasPrefix(l, "name: ") {
			name = l
		} else if strings.HasPrefix(l, "kind: ") {
			kind = l
		} else {
			sb.WriteString(l)
			sb.WriteString("\n")
		}
	}
	return fmt.Sprintf("%s\n%s\n\n%s", name, kind, sb.String()), nil
}

// GetObject gets an object.
func GetObject(cmd *cobra.Command, args *general.ArgInfo, kind string, flags *ObjectNamespaceFlags) error {
	msg := fmt.Sprintf("all %s", kind)
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetObject(args.Name, flags)
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

	if flags.AllNamespace {
		err := unmarshalPrintNamespaceMetaSpec(body, func(m *supervisor.MetaSpec) bool {
			return m.Kind == kind
		})
		if err != nil {
			return getErr(err)
		}
		return nil
	}

	err = unmarshalPrintMetaSpec(body, !args.ContainName(), func(m *supervisor.MetaSpec) bool {
		return m.Kind == kind
	})
	if err != nil {
		return getErr(err)
	}
	return nil
}

func unmarshalPrintMetaSpec(body []byte, list bool, filter func(*supervisor.MetaSpec) bool) error {
	metas, err := unmarshalMetaSpec(body, list)
	if err != nil {
		return err
	}
	if filter != nil {
		metas = general.Filter(metas, filter)
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Name < metas[j].Name
	})
	printMetaSpec(metas)
	return nil
}

func unmarshalPrintNamespaceMetaSpec(body []byte, filter func(*supervisor.MetaSpec) bool) error {
	allMetas, err := unmarshalNamespaceMetaSpec(body)
	if err != nil {
		return err
	}
	if filter != nil {
		for k, v := range allMetas {
			allMetas[k] = general.Filter(v, filter)
		}
	}
	for k, v := range allMetas {
		if len(v) > 0 {
			sort.Slice(v, func(i, j int) bool {
				return v[i].Name < v[j].Name
			})
			allMetas[k] = v
		}
	}
	printNamespaceMetaSpec(allMetas)
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

func unmarshalNamespaceMetaSpec(body []byte) (map[string][]*supervisor.MetaSpec, error) {
	res := map[string][]*supervisor.MetaSpec{}
	err := codectool.Unmarshal(body, &res)
	return res, err
}

func getAgeFromMetaSpec(meta *supervisor.MetaSpec) string {
	createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
	if err != nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(createdAt))
}

func printNamespaceMetaSpec(metas map[string][]*supervisor.MetaSpec) {
	// Output:
	// NAME KIND NAMESPACE AGE
	// ...
	table := [][]string{}
	table = append(table, []string{"NAME", "KIND", "NAMESPACE", "AGE"})
	defaults := metas[DefaultNamespace]
	for _, meta := range defaults {
		table = append(table, []string{meta.Name, meta.Kind, DefaultNamespace, getAgeFromMetaSpec(meta)})
	}
	for namespace, metas := range metas {
		if namespace == DefaultNamespace {
			continue
		}
		for _, meta := range metas {
			table = append(table, []string{meta.Name, meta.Kind, namespace, getAgeFromMetaSpec(meta)})
		}
	}
	general.PrintTable(table)
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
func DescribeObject(cmd *cobra.Command, args *general.ArgInfo, kind string, flags *ObjectNamespaceFlags) error {
	msg := fmt.Sprintf("all %s", kind)
	if args.ContainName() {
		msg = fmt.Sprintf("%s %s", kind, args.Name)
	}
	getErr := func(err error) error {
		return general.ErrorMsg(general.GetCmd, err, msg)
	}

	body, err := httpGetObject(args.Name, flags)
	if err != nil {
		return getErr(err)
	}

	namespaceSpecs := map[string][]map[string]interface{}{}
	if flags.AllNamespace {
		err := codectool.Unmarshal(body, &namespaceSpecs)
		if err != nil {
			return getErr(err)
		}
	} else {
		specs, err := general.UnmarshalMapInterface(body, !args.ContainName())
		if err != nil {
			return getErr(err)
		}
		namespace := DefaultNamespace
		if flags.Namespace != "" {
			namespace = flags.Namespace
		}
		namespaceSpecs[namespace] = specs
	}

	for namespace, specs := range namespaceSpecs {
		namespaceSpecs[namespace] = general.Filter(specs, func(m map[string]interface{}) bool {
			return m["kind"] == kind
		})
	}

	err = addObjectStatusToSpec(namespaceSpecs, args, flags)
	if err != nil {
		return getErr(err)
	}

	if !general.CmdGlobalFlags.DefaultFormat() {
		var input interface{}
		if flags.AllNamespace {
			input = namespaceSpecs
		} else {
			for _, v := range namespaceSpecs {
				input = v
			}
		}
		body, err = codectool.MarshalJSON(input)
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

	printSpace := false
	specs := namespaceSpecs[DefaultNamespace]
	if len(specs) > 0 {
		printNamespace(DefaultNamespace)
		general.PrintMapInterface(specs, specials, []string{objectStatusKeyInSpec})
		printSpace = true
	}
	for k, v := range namespaceSpecs {
		if k == DefaultNamespace {
			continue
		}
		if len(v) == 0 {
			continue
		}
		if printSpace {
			fmt.Println()
			fmt.Println()
		}
		printNamespace(k)
		general.PrintMapInterface(v, specials, []string{objectStatusKeyInSpec})
		printSpace = true
	}
	return nil
}

func printNamespace(ns string) {
	msg := fmt.Sprintf("In Namespace %s:", ns)
	fmt.Println(strings.Repeat("=", len(msg)))
	fmt.Println(msg)
	fmt.Println(strings.Repeat("=", len(msg)))
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
	body, err := httpGetObject("", nil)
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
		_, err := httpGetObject(name, nil)
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

// ObjectStatus is the status of an TrafficObject.
type ObjectStatus struct {
	Spec   map[string]interface{} `json:"spec"`
	Status map[string]interface{} `json:"status"`
}

func unmarshalObjectStatus(data []byte) (ObjectStatus, error) {
	var status ObjectStatus
	err := codectool.Unmarshal(data, &status)
	// if status.Spec and status.Status are all nil
	// then means the status is not traffic controller object status (not httpserver, pipeline).
	// we need to re-unmarshal to get true status.
	if status.Spec == nil && status.Status == nil {
		status.Status = map[string]interface{}{}
		codectool.Unmarshal(data, &status.Status)
	}
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
	Node   string                 `json:"node"`
	Status map[string]interface{} `json:"status"`
}

const objectStatusKeyInSpec = "allStatus"

func addObjectStatusToSpec(allSpecs map[string][]map[string]interface{}, args *general.ArgInfo, flags *ObjectNamespaceFlags) error {
	getUrl := func(args *general.ArgInfo) string {
		// no name, all namespaces, non default namespace, then get all status
		if !args.ContainName() || flags.AllNamespace {
			return makePath(general.StatusObjectsURL)
		}
		url := makePath(general.StatusObjectItemURL, args.Name)
		return fmt.Sprintf("%s?namespace=%s", url, flags.Namespace)
	}

	body, err := handleReq(http.MethodGet, getUrl(args), nil)
	if err != nil {
		return err
	}

	infos, err := unmarshalObjectStatusInfo(body, args.Name)
	if err != nil {
		return err
	}

	keyFn := func(namespace, name string) string {
		return namespace + "/" + name
	}
	// key is name, value is array of node and status
	status := map[string][]*NodeStatus{}
	for _, info := range infos {
		key := keyFn(info.namespace, info.name)
		status[key] = append(status[key], &NodeStatus{
			Node:   info.node,
			Status: info.status,
		})
	}

	categoryMap := func() map[string]string {
		rs, err := ObjectAPIResources()
		if err != nil {
			return map[string]string{}
		}
		res := map[string]string{}
		for _, r := range rs {
			res[r.Kind] = r.Category
		}
		return res
	}()
	getNamespace := func(kind string, namespace string) string {
		category := categoryMap[kind]
		if category == "" {
			switch kind {
			case "HTTPServer":
				category = supervisor.CategoryTrafficGate
			case "Pipeline":
				category = supervisor.CategoryPipeline
			}
		}
		if category == supervisor.CategoryPipeline || category == supervisor.CategoryTrafficGate {
			return trafficObjectStatusNamespace(namespace)
		}
		return namespace
	}
	for namespace, specs := range allSpecs {
		for i := range specs {
			spec := specs[i]
			ns := getNamespace(spec["kind"].(string), namespace)
			name := spec["name"].(string)
			allSpecs[namespace][i][objectStatusKeyInSpec] = status[keyFn(ns, name)]
		}
	}
	return nil
}
