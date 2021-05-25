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
	// Objects
	_ "github.com/megaease/easegateway/pkg/object/easemonitormetrics"
	_ "github.com/megaease/easegateway/pkg/object/function"
	_ "github.com/megaease/easegateway/pkg/object/httppipeline"
	_ "github.com/megaease/easegateway/pkg/object/httpserver"
	_ "github.com/megaease/easegateway/pkg/object/meshcontroller"
	_ "github.com/megaease/easegateway/pkg/object/serviceregistry/consulserviceregistry"
	_ "github.com/megaease/easegateway/pkg/object/serviceregistry/etcdserviceregistry"
	_ "github.com/megaease/easegateway/pkg/object/serviceregistry/eurekaserviceregistry"
	_ "github.com/megaease/easegateway/pkg/object/serviceregistry/zookeeperserviceregistry"

	// Filters
	_ "github.com/megaease/easegateway/pkg/filter/apiaggregator"
	_ "github.com/megaease/easegateway/pkg/filter/backend"
	_ "github.com/megaease/easegateway/pkg/filter/bridge"
	_ "github.com/megaease/easegateway/pkg/filter/corsadaptor"
	_ "github.com/megaease/easegateway/pkg/filter/fallback"
	_ "github.com/megaease/easegateway/pkg/filter/mock"
	_ "github.com/megaease/easegateway/pkg/filter/remotefilter"
	_ "github.com/megaease/easegateway/pkg/filter/requestadaptor"
	_ "github.com/megaease/easegateway/pkg/filter/resilience/circuitbreaker"
	_ "github.com/megaease/easegateway/pkg/filter/resilience/ratelimiter"
	_ "github.com/megaease/easegateway/pkg/filter/resilience/retryer"
	_ "github.com/megaease/easegateway/pkg/filter/resilience/timelimiter"
	_ "github.com/megaease/easegateway/pkg/filter/responseadaptor"
	_ "github.com/megaease/easegateway/pkg/filter/validator"
)
