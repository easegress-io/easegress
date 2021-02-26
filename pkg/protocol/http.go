// Package protocol gathers protocol-specific stuff,
// which eliminate the coupling of framework and implementation.
package protocol

import (
	"github.com/megaease/easegateway/pkg/context"
)

type (
	// HTTPHandler is the common handler for the all backends
	// which handle the traffic from HTTPServer.
	HTTPHandler interface {
		Handle(ctx context.HTTPContext)
	}
)
