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

package ingresscontroller

import (
	"fmt"
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/informers"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	networkingv1 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	apicorev1 "k8s.io/api/core/v1"
	apinetv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	resyncPeriod = 10 * time.Minute
)

func isResourceChanged(oldObj, newObj interface{}) bool {
	if oldObj == nil || newObj == nil {
		return true
	}

	if oldObj.(metav1.Object).GetResourceVersion() == newObj.(metav1.Object).GetResourceVersion() {
		return false
	}

	if ep, ok := oldObj.(*apicorev1.Endpoints); ok {
		return isEndpointsChanged(ep, newObj.(*apicorev1.Endpoints))
	}

	return true
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
	namespaces      []string
	clientset       *kubernetes.Clientset
	informerFactory informers.SharedInformerFactory
	eventCh         chan interface{}
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

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Errorf("error building kubernetes clientset: %s", err.Error())
		return nil, err
	}

	err = checkKubernetesVersion(cfg)
	if err != nil {
		logger.Errorf("error checking kubernetes version: %s", err.Error())
		return nil, err
	}

	return &k8sClient{
		clientset: clientset,
		eventCh:   make(chan interface{}, 1),
	}, nil
}

func checkKubernetesVersion(cfg *rest.Config) (err error) {
	cli, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		err = fmt.Errorf("failed to get kubernetes version: %v", err.Error())
		return
	}

	info, err := cli.ServerVersion()
	if err != nil {
		return
	}
	if info.Major != "1" {
		err = fmt.Errorf("unknown kubernetes major version: %v", info.Major)
		return
	}

	minor, err := strconv.Atoi(info.Minor)
	if err != nil {
		err = fmt.Errorf("unknown kubernetes minor version: %v, %s", info.Minor, err.Error())
		return
	}

	if minor < 19 {
		// Ingress version v1 has been added after kubernetes 1.19
		panic(fmt.Errorf("kubernetes version [%v] is too low, IngressController requires kubernetes v1.19+", info.GitVersion))
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

	factory := informers.NewSharedInformerFactory(c.clientset, resyncPeriod)
	for _, ns := range namespaces {
		informer := networkingv1.New(factory, ns, nil).Ingresses().Informer()
		informer.AddEventHandler(c)

		informer = corev1.New(factory, ns, nil).Services().Informer()
		informer.AddEventHandler(c)

		informer = corev1.New(factory, ns, nil).Endpoints().Informer()
		informer.AddEventHandler(c)

		informer = corev1.New(factory, ns, internalinterfaces.TweakListOptionsFunc(notHelm)).Secrets().Informer()
		informer.AddEventHandler(c)
	}

	factory.Start(stopCh)
	for typ, ok := range factory.WaitForCacheSync(stopCh) {
		if !ok {
			close(stopCh)
			return nil, fmt.Errorf("timed out waiting for controller caches to sync %s", typ)
		}
	}

	c.informerFactory = factory
	return stopCh, nil
}

func (c *k8sClient) getService(namespace, name string) (*apicorev1.Service, error) {
	service, err := c.informerFactory.Core().V1().Services().Lister().Services(namespace).Get(name)
	if errors.IsNotFound(err) {
		err = nil
	}
	return service, err
}

func (c *k8sClient) getEndpoints(namespace, name string) (*apicorev1.Endpoints, error) {
	endpoint, err := c.informerFactory.Core().V1().Endpoints().Lister().Endpoints(namespace).Get(name)
	if errors.IsNotFound(err) {
		err = nil
	}
	return endpoint, err
}

func (c *k8sClient) getSecret(namespace, name string) (*apicorev1.Secret, error) {
	secret, err := c.informerFactory.Core().V1().Secrets().Lister().Secrets(namespace).Get(name)
	if errors.IsNotFound(err) {
		err = nil
	}
	return secret, err
}

func (c *k8sClient) getIngresses(ingressClass string) []*apinetv1.Ingress {
	var result []*apinetv1.Ingress

	lister := c.informerFactory.Networking().V1().Ingresses().Lister()
	for _, ns := range c.namespaces {
		list, err := lister.Ingresses(ns).List(labels.Everything())
		if err != nil {
			logger.Errorf("Failed to get ingresses from namespace %s: %v", ns, err)
			continue
		}

		for _, ingress := range list {
			var ic string
			if ingress.Spec.IngressClassName == nil {
				ic = ingress.Annotations[k8sIngressClassAnnotation]
			} else {
				ic = *ingress.Spec.IngressClassName
			}
			if ic == ingressClass {
				result = append(result, ingress)
			}
		}
	}

	return result
}
