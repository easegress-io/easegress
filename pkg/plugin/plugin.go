package plugin

import "github.com/megaease/easegateway/pkg/context"

type (
	// Plugin contains all methods Plugin must satisfy.
	Plugin interface {
		Close()
	}

	// HTTPPlugin contains all methods HTTPPlugin must satisfy.
	HTTPPlugin interface {
		Plugin
		Handle(ctx context.HTTPContext)
	}
)
