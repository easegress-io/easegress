package aigatewaycontroller

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/api"
)

const (
	APIGroupName = "ai_gateway"
	APIPrefix    = "/ai-gateway"
)

func (agc *AIGatewayController) registerAPIs() {
	group := &api.Group{
		Group: APIGroupName,
		Entries: []*api.Entry{
			{Path: APIPrefix + "/providers/status", Method: "GET", Handler: agc.checkProvidersStatus},
			{Path: APIPrefix + "/stat", Method: "GET", Handler: agc.stat},
		},
	}

	api.RegisterAPIs(group)
}

func (agc *AIGatewayController) unregisterAPIs() {
	api.UnregisterAPIs(APIGroupName)
}

func (agc *AIGatewayController) checkProvidersStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement the logic to check the status of AI providers.
}

func (agc *AIGatewayController) stat(w http.ResponseWriter, r *http.Request) {
	// TODO: Gather statistics for the AI Gateway.
	// Statistics items:
	// - Pipelines containing AIFilter.
	// - Total requests processed
	// - Total successful responses
	// - Total error responses
	// - etc.
}
