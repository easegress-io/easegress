package registry

import (
	// Objects
	_ "github.com/megaease/easegateway/pkg/object/httppipeline"
	_ "github.com/megaease/easegateway/pkg/object/httpproxy"
	_ "github.com/megaease/easegateway/pkg/object/httpserver"

	// Plugins
	_ "github.com/megaease/easegateway/pkg/plugin/backend"
	_ "github.com/megaease/easegateway/pkg/plugin/fallback"
	_ "github.com/megaease/easegateway/pkg/plugin/mock"
	_ "github.com/megaease/easegateway/pkg/plugin/ratelimiter"
	_ "github.com/megaease/easegateway/pkg/plugin/remoteplugin"
	_ "github.com/megaease/easegateway/pkg/plugin/requestadaptor"
	_ "github.com/megaease/easegateway/pkg/plugin/responseadaptor"
	_ "github.com/megaease/easegateway/pkg/plugin/validator"
)
