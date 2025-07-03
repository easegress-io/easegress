package aigatewaycontroller

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	APIGroupName = "ai_gateway"
	APIPrefix    = "/ai-gateway"
)

type (
	HealthCheckResponse struct {
		Results []HealthCheckResult `json:"results"`
	}

	HealthCheckResult struct {
		Name         string `json:"name,omitempty"`
		ProviderType string `json:"providerType"`
		Healthy      bool   `json:"healthy"`
		Error        string `json:"error,omitempty"`
	}

	StatsResponse struct {
		Stats []*metricshub.MetricStats `json:"stats"`
	}
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
	resp := HealthCheckResponse{}
	for _, provider := range agc.providers {
		result := HealthCheckResult{
			Name:         provider.Name(),
			ProviderType: provider.Type(),
			Healthy:      true,
		}

		err := provider.HealthCheck()
		if err != nil {
			result.Healthy = false
			result.Error = err.Error()
		}

		resp.Results = append(resp.Results, result)
	}

	w.Write(codectool.MustMarshalJSON(resp))
}

func (agc *AIGatewayController) stat(w http.ResponseWriter, r *http.Request) {
	stats := agc.metricshub.GetStats()
	resp := StatsResponse{
		Stats: stats,
	}
	w.Write(codectool.MustMarshalJSON(resp))
}
