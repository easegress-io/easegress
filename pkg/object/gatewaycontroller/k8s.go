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

package gatewaycontroller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	kinformers "k8s.io/client-go/informers"
	kclientset "k8s.io/client-go/kubernetes"

	gwapis "sigs.k8s.io/gateway-api/apis/v1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	gwinformerapis "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"
)

const (
	resyncPeriod = 10 * time.Minute
)

func isResourceChanged(oldObj, newObj interface{}) bool {
	if oldObj == nil || newObj == nil {
		return true
	}

	if oldObj.(metav1.Object).GetResourceVersion() != newObj.(metav1.Object).GetResourceVersion() {
		return true
	}

	if ep, ok := oldObj.(*apicorev1.Endpoints); ok {
		return isEndpointsChanged(ep, newObj.(*apicorev1.Endpoints))
	}

	return false
}

func isEndpointsChanged(a, b *apicorev1.Endpoints) bool {
	if len(a.Subsets) != len(b.Subsets) {
		return true
	}

	for i, sa := range a.Subsets {
		sb := b.Subsets[i]
		if isSubsetsChanged(sa, sb) {
			return true
		}
	}

	return false
}

func isSubsetsChanged(a, b apicorev1.EndpointSubset) bool {
	if len(a.Addresses) != len(b.Addresses) {
		return true
	}

	if len(a.Ports) != len(b.Ports) {
		return true
	}

	for i, aa := range a.Addresses {
		ba := b.Addresses[i]
		if aa.IP != ba.IP {
			return true
		}
		if aa.Hostname != ba.Hostname {
			return true
		}
	}

	for i, pa := range a.Ports {
		pb := b.Ports[i]
		if pa.Name != pb.Name {
			return true
		}
		if pa.Port != pb.Port {
			return true
		}
		if pa.Protocol != pb.Protocol {
			return true
		}
	}

	return false
}

type k8sClient struct {
	namespaces []string

	kcs      *kclientset.Clientset
	kFactory kinformers.SharedInformerFactory

	gwcs      *gwclientset.Clientset
	gwFactory gwinformers.SharedInformerFactory

	dc *dynamic.DynamicClient

	eventCh chan interface{}
}

// OnAdd is called on Resource Add Events.
func (c *k8sClient) OnAdd(obj interface{}, isInInitialList bool) {
	// if there's an event already in the channel, discard this one,
	// this is fine because IngressController always reload everything
	// when receiving an event. Same for OnUpdate & OnDelete
	select {
	case c.eventCh <- obj:
	default:
	}
}

// OnUpdate is called on Resource Update Events.
func (c *k8sClient) OnUpdate(oldObj, newObj interface{}) {
	if !isResourceChanged(oldObj, newObj) {
		return
	}

	select {
	case c.eventCh <- newObj:
	default:
	}
}

// OnDelete is called on Resource Delete Events.
func (c *k8sClient) OnDelete(obj interface{}) {
	select {
	case c.eventCh <- obj:
	default:
	}
}

func newK8sClient(masterURL string, kubeConfig string) (*k8sClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		logger.Errorf("error building kubeconfig: %s", err.Error())
		return nil, err
	}

	kcs, err := kclientset.NewForConfig(cfg)
	if err != nil {
		logger.Errorf("error building kubernetes clientset: %s", err.Error())
		return nil, err
	}

	gwcs, err := gwclientset.NewForConfig(cfg)
	if err != nil {
		logger.Errorf("error building gateway clientset: %s", err.Error())
		return nil, err
	}

	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Errorf("error building dynamic clientset: %s", err.Error())
		return nil, err
	}

	err = checkKubernetesVersion(cfg)
	if err != nil {
		logger.Errorf("error checking kubernetes version: %s", err.Error())
		return nil, err
	}

	return &k8sClient{
		kcs:     kcs,
		gwcs:    gwcs,
		dc:      dc,
		eventCh: make(chan interface{}, 1),
	}, nil
}

func checkKubernetesVersion(cfg *rest.Config) (err error) {
	cli, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		err = fmt.Errorf("failed to discover kubernetes client config: %v", err.Error())
		return
	}

	info, err := cli.ServerVersion()
	if err != nil {
		err = fmt.Errorf("failed to get kubernetes version: %v", err.Error())
		return
	}

	if info.Major != "1" {
		err = fmt.Errorf("unknown kubernetes major version: %v", info.Major)
		return
	}

	minor, err := strconv.Atoi(info.Minor)
	if err != nil {
		err = fmt.Errorf("unknown kubernetes minor version: %v", info.Minor)
		return
	}

	if minor < 23 {
		// Gateway API documentation says it support at least 5 minor versions
		// of k8s, and now k8s version is 1.27.x.
		panic(fmt.Errorf("kubernetes version [%v] is too low, GatewayController requires kubernetes v1.23+", info.GitVersion))
	}

	return
}

func (c *k8sClient) event() <-chan interface{} {
	return c.eventCh
}

func (c *k8sClient) watch(namespaces []string) (chan struct{}, error) {
	stopCh := make(chan struct{})

	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}
	c.namespaces = namespaces

	notHelm := func(opts *metav1.ListOptions) {
		opts.LabelSelector = "owner!=helm"
	}

	/*
		 labelSelectorOptions := func(options *metav1.ListOptions) {
			 options.LabelSelector = c.labelSelector
		 }
	*/

	c.kFactory = kinformers.NewSharedInformerFactory(c.kcs, resyncPeriod)
	_, err := c.kFactory.Core().V1().Namespaces().Informer().AddEventHandler(c)
	if err != nil {
		return nil, err
	}

	c.gwFactory = gwinformers.NewSharedInformerFactoryWithOptions(c.gwcs, resyncPeriod, gwinformers.WithTweakListOptions(notHelm))
	_, err = c.gwFactory.Gateway().V1().GatewayClasses().Informer().AddEventHandler(c)
	if err != nil {
		return nil, err
	}

	dFactories := []dynamicinformer.DynamicSharedInformerFactory{}
	for _, ns := range namespaces {
		gwinformerapis.New(c.gwFactory, ns, nil).Gateways().Informer().AddEventHandler(c)
		gwinformerapis.New(c.gwFactory, ns, nil).HTTPRoutes().Informer().AddEventHandler(c)
		// we can not handle tcp route and tls route currently

		corev1.New(c.kFactory, ns, nil).Services().Informer().AddEventHandler(c)
		corev1.New(c.kFactory, ns, nil).Endpoints().Informer().AddEventHandler(c)
		corev1.New(c.kFactory, ns, internalinterfaces.TweakListOptionsFunc(notHelm)).Secrets().Informer().AddEventHandler(c)

		dFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(c.dc, resyncPeriod, ns, nil)
		dFactory.ForResource(FilterSpecGVR).Informer().AddEventHandler(c)
		dFactories = append(dFactories, dFactory)
	}

	c.kFactory.Start(stopCh)
	c.gwFactory.Start(stopCh)
	for _, f := range dFactories {
		f.Start(stopCh)
	}

	for typ, ok := range c.kFactory.WaitForCacheSync(stopCh) {
		if !ok {
			close(stopCh)
			return nil, fmt.Errorf("timed out waiting for k8s caches to sync %s", typ)
		}
	}

	for typ, ok := range c.gwFactory.WaitForCacheSync(stopCh) {
		if !ok {
			close(stopCh)
			return nil, fmt.Errorf("timed out waiting for gateway caches to sync %s", typ)
		}
	}

	return stopCh, nil
}

func (c *k8sClient) GetHTTPRoutes() []*gwapis.HTTPRoute {
	var results []*gwapis.HTTPRoute

	lister := c.gwFactory.Gateway().V1().HTTPRoutes().Lister()
	for _, ns := range c.namespaces {
		if !c.isNamespaceWatched(ns) {
			logger.Warnf("failed to get HTTPRoutes: %q is not within watched namespaces", ns)
			continue
		}

		routes, err := lister.HTTPRoutes(ns).List(labels.Everything())
		if err != nil {
			logger.Errorf("failed to list HTTPRoute in namespace %q: %v", ns, err)
			continue
		}

		if len(routes) == 0 {
			logger.Debugf("no HTTPRoutes found in namespace %q", ns)
			continue
		}

		results = append(results, routes...)
	}

	return results
}

func (c *k8sClient) GetGateways() []*gwapis.Gateway {
	var result []*gwapis.Gateway

	lister := c.gwFactory.Gateway().V1().Gateways().Lister()
	for _, ns := range c.namespaces {
		gateways, err := lister.Gateways(ns).List(labels.Everything())
		if err != nil {
			logger.Errorf("failed to list Gateways in namespace %s: %v", ns, err)
			continue
		}

		result = append(result, gateways...)
	}

	return result
}

func (c *k8sClient) GetGatewayClasses(controllerName string) []*gwapis.GatewayClass {
	classes, err := c.gwFactory.Gateway().V1().GatewayClasses().Lister().List(labels.Everything())
	if err != nil {
		logger.Errorf("Failed to list GatewayClasses: %v", err)
		return nil
	}

	var result []*gwapis.GatewayClass
	for _, c := range classes {
		if string(c.Spec.ControllerName) == controllerName {
			result = append(result, c)
		}
	}

	return result
}

func (c *k8sClient) UpdateGatewayClassStatus(gatewayClass *gwapis.GatewayClass, status gwapis.GatewayClassStatus) error {
	gc := gatewayClass.DeepCopy()
	gc.Status = status

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.gwcs.GatewayV1().GatewayClasses().UpdateStatus(ctx, gc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GatewayClass %s/%s status: %w", gatewayClass.Namespace, gatewayClass.Name, err)
	}
	return nil
}

func isStatusChanged(old, new gwapis.GatewayStatus) bool {
	if len(old.Listeners) != len(new.Listeners) {
		return true
	}

	if isConditionsChanged(old.Conditions, new.Conditions) {
		return true
	}

	matches := 0
	for _, nl := range new.Listeners {
		for _, ol := range old.Listeners {
			if nl.Name == ol.Name {
				if isConditionsChanged(nl.Conditions, ol.Conditions) {
					return true
				}

				matches++
			}
		}
	}

	return matches != len(old.Listeners)
}

func isConditionsChanged(new, old []metav1.Condition) bool {
	if len(new) != len(old) {
		return true
	}

	matches := 0
	for _, n := range new {
		for _, o := range old {
			if n.Type == o.Type {
				continue
			}

			if n.Reason != o.Reason {
				return true
			}
			if n.Status != o.Status {
				return true
			}
			if n.Message != o.Message {
				return true
			}
			matches++
		}
	}

	return matches != len(old)
}

func (c *k8sClient) UpdateGatewayStatus(gateway *gwapis.Gateway, status gwapis.GatewayStatus) error {
	if !c.isNamespaceWatched(gateway.Namespace) {
		return fmt.Errorf("cannot update Gateway status %s/%s: namespace is not within watched namespaces", gateway.Namespace, gateway.Name)
	}

	if !isStatusChanged(gateway.Status, status) {
		return nil
	}

	g := gateway.DeepCopy()
	g.Status = status

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.gwcs.GatewayV1().Gateways(gateway.Namespace).UpdateStatus(ctx, g, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update Gateway %q status: %w", gateway.Name, err)
	}

	return nil
}

func (c *k8sClient) getService(namespace, name string) (*apicorev1.Service, error) {
	service, err := c.kFactory.Core().V1().Services().Lister().Services(namespace).Get(name)
	if errors.IsNotFound(err) {
		err = nil
	}
	return service, err
}

func (c *k8sClient) getSecret(namespace, name string) (*apicorev1.Secret, error) {
	secret, err := c.kFactory.Core().V1().Secrets().Lister().Secrets(namespace).Get(name)
	if errors.IsNotFound(err) {
		err = nil
	}
	return secret, err
}

func (c *k8sClient) isNamespaceWatched(ns string) bool {
	if len(c.namespaces) == 0 {
		return true
	}
	for _, watchedNamespace := range c.namespaces {
		if watchedNamespace == metav1.NamespaceAll {
			return true
		}
		if watchedNamespace == ns {
			return true
		}
	}
	return false
}

// FilterSpecFromCR is filter spec from kubernetes custom resource.
type FilterSpecFromCR struct {
	Name string
	Kind string
	Spec string
}

var FilterSpecGVR = schema.GroupVersionResource{
	Group:    "easegress.megaease.com",
	Version:  "v1",
	Resource: "filterspecs",
}

// GetFilterSpecFromCustomResource get filter spec from kubernetes custom resource.
func (c *k8sClient) GetFilterSpecFromCustomResource(namespace string, objName string) (*FilterSpecFromCR, error) {
	cr, err := c.dc.Resource(FilterSpecGVR).Namespace(namespace).Get(context.Background(), objName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	crSpec := cr.Object["spec"].(map[string]interface{})
	name, ok := crSpec["name"].(string)
	if !ok {
		return nil, fmt.Errorf("custom resource %v/%s does not have string name field", FilterSpecGVR, objName)
	}
	kind, ok := crSpec["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("custom resource %v/%s does not have string kind field", FilterSpecGVR, objName)
	}
	spec, ok := crSpec["spec"].(string)
	if !ok {
		return nil, fmt.Errorf("custom resource %v/%s does not have string spec field", FilterSpecGVR, objName)
	}
	return &FilterSpecFromCR{
		Name: name,
		Kind: kind,
		Spec: spec,
	}, nil
}
