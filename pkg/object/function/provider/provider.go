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

// Package provider defines and implements FaasProvider interface.
package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	k8sapisv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/client/pkg/kn/commands"
	clientservingv1 "knative.dev/client/pkg/serving/v1"
	"knative.dev/serving/pkg/apis/autoscaling"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/function/spec"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

type (
	// FaaSProvider is the physical serverless function instance manager
	FaaSProvider interface {
		Init() error
		GetStatus(name string) (*spec.Status, error)
		Create(funcSpec *spec.Spec) error
		Delete(name string) error
		Update(funcSpec *spec.Spec) error
	}

	// knativeClient is client for communicating with Knative type FaaSProvider
	knativeClient struct {
		superSpec     *supervisor.Spec
		serviceClient clientservingv1.KnServingClient
		namespace     string
		timeout       time.Duration
	}
)

func (kc *knativeClient) Create(funcSpec *spec.Spec) error {
	return kc.createService(funcSpec)
}

func (kc *knativeClient) GetStatus(name string) (*spec.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kc.timeout)
	defer cancel()

	service, err := kc.serviceClient.GetService(ctx, name)
	if err != nil {
		logger.Errorf("knative get service: %s, err: %v", name, err)
		return nil, err
	}
	status := &spec.Status{}
	extData := map[string]string{}
	hasErrors := false

	if len(service.Status.LatestReadyRevisionName) == 0 ||
		service.Status.LatestCreatedRevisionName != service.Status.LatestReadyRevisionName {
		for _, v := range service.Status.Conditions {
			// There are three types of condition, false, unknown, true
			if v.Status == corev1.ConditionFalse {
				hasErrors = true
			}
			key := fmt.Sprintf("%v", v.Type)
			value := fmt.Sprintf("status: %v, message: %v, reason: %v", v.Status, v.Message, v.Reason)
			extData[key] = value
		}
		status.ExtData = extData

		if hasErrors {
			status.Event = spec.ErrorEvent
		} else {
			status.Event = spec.PendingEvent
		}
	} else {
		// only when latestCreateRevisionName equals with latestReadyRevisionName, then
		// we can consider this knative service is ready for handling traffic.
		status.Event = spec.ReadyEvent
	}

	return status, nil
}

func (kc *knativeClient) Update(spec *spec.Spec) error {
	return kc.updateService(spec)
}

func (kc *knativeClient) Delete(name string) error {
	return kc.deleteService(name)
}

// NewProvider returns FaaSProvider client. It only supports Knative now.
func NewProvider(superSpec *supervisor.Spec) FaaSProvider {
	return &knativeClient{
		superSpec: superSpec,
	}
}

// Init initializes knative client.
func (kc *knativeClient) Init() error {
	var err error
	param := &commands.KnParams{}
	param.Initialize()
	spec := kc.superSpec.ObjectSpec().(*spec.Admin)

	kc.namespace = spec.Knative.Namespace
	kc.serviceClient, err = param.NewServingClient(kc.namespace)
	if err != nil {
		logger.Errorf("knative new serving client failed: %v", err)
		return err
	}

	kc.timeout, err = time.ParseDuration(spec.Knative.Timeout)
	if err != nil {
		logger.Errorf("BUG: parse knative timeout interval: %s failed: %v",
			spec.Knative.Timeout, err)
		return err
	}
	return nil
}

// For scaling about annotations
func annotation(funcSpec *spec.Spec) map[string]string {
	annotation := make(map[string]string)
	switch funcSpec.AutoScaleType {
	case spec.AutoScaleMetricCPU:
		annotation[autoscaling.ClassAnnotationKey] = autoscaling.HPA
		annotation[autoscaling.MetricAnnotationKey] = autoscaling.CPU
	case spec.AutoScaleMetricConcurrency:
		annotation[autoscaling.MetricAnnotationKey] = autoscaling.Concurrency
	case spec.AutoScaleMetricRPS:
	default:
		// using RPS default
		annotation[autoscaling.MetricAnnotationKey] = autoscaling.RPS
	}
	annotation[autoscaling.TargetAnnotationKey] = funcSpec.AutoScaleValue
	if funcSpec.MinReplica != 0 {
		annotation[autoscaling.MinScaleAnnotationKey] = strconv.Itoa(funcSpec.MinReplica)
	}

	if funcSpec.MaxReplica != 0 {
		annotation[autoscaling.MaxScaleAnnotationKey] = strconv.Itoa(funcSpec.MaxReplica)
	}

	return annotation
}

// container builds a container with a dedicated port and image.
func container(funcSpec *spec.Spec) corev1.Container {
	container := corev1.Container{
		Image:     funcSpec.Image,
		Resources: requirement(funcSpec),
	}

	if funcSpec.Port != 0 {
		container.Ports = []corev1.ContainerPort{{
			ContainerPort: int32(funcSpec.Port),
		}}
	}
	return container
}

// requirement builds requirement according to resource's limitation and requested
func requirement(funcSpec *spec.Spec) corev1.ResourceRequirements {
	rr := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: k8sresource.MustParse(funcSpec.LimitMemory),
			corev1.ResourceCPU:    k8sresource.MustParse(funcSpec.LimitCPU),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    k8sresource.MustParse(funcSpec.RequestCPU),
			corev1.ResourceMemory: k8sresource.MustParse(funcSpec.RequestMemory),
		},
	}
	return rr
}

func applySpec(funcSpec *spec.Spec, service *servingv1.Service) {
	container := container(funcSpec)
	service.Spec.Template.Spec.Containers = []corev1.Container{container}
	service.Spec.Template.ObjectMeta.Annotations = annotation(funcSpec)
}

// newService generate a knative service resource by giving Spec
func (kc *knativeClient) newService(funcSpec *spec.Spec) *servingv1.Service {
	service := &servingv1.Service{
		ObjectMeta: k8sapisv1.ObjectMeta{
			Name:      funcSpec.Name,
			Namespace: kc.namespace,
		},
	}

	service.Spec.Template = servingv1.RevisionTemplateSpec{
		Spec: servingv1.RevisionSpec{
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{container(funcSpec)},
			},
		},
		ObjectMeta: k8sapisv1.ObjectMeta{
			Annotations: annotation(funcSpec),
		},
	}
	return service
}

// createService creates a knative service resource.
func (kc *knativeClient) createService(funcSpec *spec.Spec) error {
	service := kc.newService(funcSpec)
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), kc.timeout)
	defer cancel()
	err = kc.serviceClient.CreateService(ctx, service)
	if err != nil {
		logger.Errorf("create knative service:%s, timeout: %v, failed: %v", funcSpec.Name, kc.timeout, err)
		return err
	}
	return nil
}

// deleteService deletes knative service resource.
func (kc *knativeClient) deleteService(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), kc.timeout)
	defer cancel()
	if err := kc.serviceClient.DeleteService(ctx, name, 0); err != nil {
		logger.Errorf("delete knative service: %s, timeout: %v, failed: %v", name, kc.timeout, err)
		return err
	}
	return nil
}

// updateService updates function's Knative and EG resources
func (kc *knativeClient) updateService(funcSpec *spec.Spec) error {
	ctx, cancel := context.WithTimeout(context.Background(), kc.timeout)
	defer cancel()
	service, err := kc.serviceClient.GetService(ctx, funcSpec.Name)
	if err != nil {
		logger.Errorf("get knative service: %s, timeout: %v, failed: %v", funcSpec.Name, kc.timeout, err)
		return err
	}

	applySpec(funcSpec, service)

	ctx1, cancel1 := context.WithTimeout(context.Background(), kc.timeout)
	defer cancel1()
	if _, err := kc.serviceClient.UpdateService(ctx1, service); err != nil {
		logger.Errorf("update update service: %s, timeout: %v, failed: %v", funcSpec.Name, kc.timeout, err)
		return err
	}
	return nil
}
