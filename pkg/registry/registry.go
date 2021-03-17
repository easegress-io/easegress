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
	_ "github.com/megaease/easegateway/pkg/filter/urlratelimiter"
	_ "github.com/megaease/easegateway/pkg/filter/validator"
)
