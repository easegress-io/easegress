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

// Package mqttproxy implements the MQTTProxy.
package mqttproxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Category is the category of MQTTProxy.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of MQTTProxy.
	Kind = "MQTTProxy"
)

var _ supervisor.TrafficObject = (*MQTTProxy)(nil)

func init() {
	supervisor.Register(&MQTTProxy{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"mqtt", "mp"},
	})
}

type (
	// MQTTProxy implements MQTT proxy in EG
	MQTTProxy struct {
		superSpec *supervisor.Spec
		spec      *Spec
		broker    *Broker
	}
)

// Category returns the category of MQTTProxy.
func (mp *MQTTProxy) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of MQTTProxy.
func (mp *MQTTProxy) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MQTTProxy.
func (mp *MQTTProxy) DefaultSpec() interface{} {
	return &Spec{}
}

// Status returns the Status of MQTTProxy.
func (mp *MQTTProxy) Status() *supervisor.Status {
	return &supervisor.Status{}
}

func updatePort(urlStr string, hostWithPort string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("parse url %v failed: %v", urlStr, err)
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", fmt.Errorf("split host for url %v failed: %v", urlStr, err)
	}
	_, port, err := net.SplitHostPort(hostWithPort)
	if err != nil {
		return "", fmt.Errorf("split host for hostWithPort %v failed: %v", hostWithPort, err)
	}
	u.Host = net.JoinHostPort(host, port)
	return u.String(), nil
}

func memberURLFunc(superSpec *supervisor.Spec) func(string, string) (map[string]string, error) {
	c := superSpec.Super().Cluster()

	f := func(egName, name string) (map[string]string, error) {
		logger.SpanDebugf(nil, "get member url for %v %v", egName, name)
		kv, err := c.GetPrefix(c.Layout().StatusMemberPrefix())
		if err != nil {
			logger.SpanErrorf(nil, "cluster get member list failed: %v", err)
			return map[string]string{}, err
		}
		// urls := []string{}
		urls := make(map[string]string)
		for _, v := range kv {
			memberStatus := cluster.MemberStatus{}
			err := codectool.Unmarshal([]byte(v), &memberStatus)
			if err != nil {
				logger.SpanErrorf(nil, "cluster status unmarshal failed: %v", err)
				return map[string]string{}, err
			}
			if memberStatus.Options.Name != egName {
				egURLs := memberStatus.Options.Cluster.InitialAdvertisePeerURLs
				peerURLVariableName := "ClusterInitialAdvertisePeerURLs"
				if memberStatus.Options.UseInitialCluster() {
					egURLs = memberStatus.Options.Cluster.InitialAdvertisePeerURLs
					peerURLVariableName = "Cluster.InitialAdvertisePeerURLs"
				}
				if len(egURLs) == 0 {
					return nil, fmt.Errorf("easegress %v has empty %v %v", memberStatus.Options.Name, peerURLVariableName, egURLs)
				}
				egURL := egURLs[0]
				apiAddr := memberStatus.Options.APIAddr
				newURL, err := updatePort(egURL, apiAddr)
				if err != nil {
					return nil, fmt.Errorf("get url for %v failed: %v", memberStatus.Options.Name, err)
				}
				// urls = append(urls, newURL+"/apis/v2"+fmt.Sprintf(mqttAPITopicPublishPrefix, name))
				urls[memberStatus.Options.Name] = newURL + "/apis/v2" + fmt.Sprintf(mqttAPITopicPublishPrefix, name)
			}
		}
		logger.SpanDebugf(nil, "eg %v %v get urls %v", egName, name, urls)
		return urls, nil
	}
	return f
}

// Init initializes Function.
func (mp *MQTTProxy) Init(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)
	spec.Name = superSpec.Name()
	spec.EGName = superSpec.Super().Options().Name
	mp.superSpec, mp.spec = superSpec, spec

	store := newStorage(superSpec.Super().Cluster())
	mp.broker = newBroker(spec, store, muxMapper, memberURLFunc(superSpec))
	if mp.broker == nil {
		panic(fmt.Sprintf("broker %v start failed", spec.Name))
	}
	mp.broker.registerAPIs()
}

// Inherit inherits previous generation of MQTTProxy.
func (mp *MQTTProxy) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper context.MuxMapper) {
	previousGeneration.Close()
	mp.Init(superSpec, muxMapper)
}

// Close closes MQTTProxy.
func (mp *MQTTProxy) Close() {
	mp.broker.close()
}
