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

package registry

import (

	// Filters
	_ "github.com/megaease/easegress/pkg/filter/apiaggregator"
	_ "github.com/megaease/easegress/pkg/filter/bridge"
	_ "github.com/megaease/easegress/pkg/filter/circuitbreaker"
	_ "github.com/megaease/easegress/pkg/filter/corsadaptor"
	_ "github.com/megaease/easegress/pkg/filter/fallback"
	_ "github.com/megaease/easegress/pkg/filter/mock"
	_ "github.com/megaease/easegress/pkg/filter/proxy"
	_ "github.com/megaease/easegress/pkg/filter/ratelimiter"
	_ "github.com/megaease/easegress/pkg/filter/remotefilter"
	_ "github.com/megaease/easegress/pkg/filter/requestadaptor"
	_ "github.com/megaease/easegress/pkg/filter/responseadaptor"
	_ "github.com/megaease/easegress/pkg/filter/retryer"
	_ "github.com/megaease/easegress/pkg/filter/timelimiter"
	_ "github.com/megaease/easegress/pkg/filter/validator"
	_ "github.com/megaease/easegress/pkg/filter/wasmhost"

	// Objects
	_ "github.com/megaease/easegress/pkg/object/consulserviceregistry"
	_ "github.com/megaease/easegress/pkg/object/easemonitormetrics"
	_ "github.com/megaease/easegress/pkg/object/etcdserviceregistry"
	_ "github.com/megaease/easegress/pkg/object/eurekaserviceregistry"
	_ "github.com/megaease/easegress/pkg/object/function"
	_ "github.com/megaease/easegress/pkg/object/httppipeline"
	_ "github.com/megaease/easegress/pkg/object/httpserver"
	_ "github.com/megaease/easegress/pkg/object/ingresscontroller"
	_ "github.com/megaease/easegress/pkg/object/meshcontroller"
	_ "github.com/megaease/easegress/pkg/object/mqttproxy"
	_ "github.com/megaease/easegress/pkg/object/nacosserviceregistry"
	_ "github.com/megaease/easegress/pkg/object/rawconfigtrafficcontroller"
	_ "github.com/megaease/easegress/pkg/object/trafficcontroller"
	_ "github.com/megaease/easegress/pkg/object/websocketserver"
	_ "github.com/megaease/easegress/pkg/object/zookeeperserviceregistry"
)
