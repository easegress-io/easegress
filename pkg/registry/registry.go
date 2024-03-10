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

// Package registry is the registry of filters and objects in Easegress.
package registry

import (
	// Filters
	_ "github.com/megaease/easegress/v2/pkg/filters/builder"
	_ "github.com/megaease/easegress/v2/pkg/filters/certextractor"
	_ "github.com/megaease/easegress/v2/pkg/filters/connectcontrol"
	_ "github.com/megaease/easegress/v2/pkg/filters/corsadaptor"
	_ "github.com/megaease/easegress/v2/pkg/filters/fallback"
	_ "github.com/megaease/easegress/v2/pkg/filters/headerlookup"
	_ "github.com/megaease/easegress/v2/pkg/filters/headertojson"
	_ "github.com/megaease/easegress/v2/pkg/filters/kafka"
	_ "github.com/megaease/easegress/v2/pkg/filters/kafkabackend"
	_ "github.com/megaease/easegress/v2/pkg/filters/meshadaptor"
	_ "github.com/megaease/easegress/v2/pkg/filters/mock"
	_ "github.com/megaease/easegress/v2/pkg/filters/mqttclientauth"
	_ "github.com/megaease/easegress/v2/pkg/filters/oidcadaptor"
	_ "github.com/megaease/easegress/v2/pkg/filters/opafilter"
	_ "github.com/megaease/easegress/v2/pkg/filters/proxies/grpcproxy"
	_ "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	_ "github.com/megaease/easegress/v2/pkg/filters/ratelimiter"
	_ "github.com/megaease/easegress/v2/pkg/filters/redirector"
	_ "github.com/megaease/easegress/v2/pkg/filters/remotefilter"
	_ "github.com/megaease/easegress/v2/pkg/filters/topicmapper"
	_ "github.com/megaease/easegress/v2/pkg/filters/validator"
	_ "github.com/megaease/easegress/v2/pkg/filters/wasmhost"

	// Objects
	_ "github.com/megaease/easegress/v2/pkg/object/autocertmanager"
	_ "github.com/megaease/easegress/v2/pkg/object/consulserviceregistry"
	_ "github.com/megaease/easegress/v2/pkg/object/easemonitormetrics"
	_ "github.com/megaease/easegress/v2/pkg/object/etcdserviceregistry"
	_ "github.com/megaease/easegress/v2/pkg/object/eurekaserviceregistry"
	_ "github.com/megaease/easegress/v2/pkg/object/function"
	_ "github.com/megaease/easegress/v2/pkg/object/gatewaycontroller"
	_ "github.com/megaease/easegress/v2/pkg/object/globalfilter"
	_ "github.com/megaease/easegress/v2/pkg/object/grpcserver"
	_ "github.com/megaease/easegress/v2/pkg/object/httpserver"
	_ "github.com/megaease/easegress/v2/pkg/object/ingresscontroller"
	_ "github.com/megaease/easegress/v2/pkg/object/meshcontroller"
	_ "github.com/megaease/easegress/v2/pkg/object/mock"
	_ "github.com/megaease/easegress/v2/pkg/object/mqttproxy"
	_ "github.com/megaease/easegress/v2/pkg/object/nacosserviceregistry"
	_ "github.com/megaease/easegress/v2/pkg/object/pipeline"
	_ "github.com/megaease/easegress/v2/pkg/object/rawconfigtrafficcontroller"
	_ "github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	_ "github.com/megaease/easegress/v2/pkg/object/zookeeperserviceregistry"

	// Routers
	_ "github.com/megaease/easegress/v2/pkg/object/httpserver/routers/ordered"
	_ "github.com/megaease/easegress/v2/pkg/object/httpserver/routers/radixtree"
)
