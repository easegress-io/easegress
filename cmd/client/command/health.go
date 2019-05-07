package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

// HealthCmd defines health command.
func HealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Probe EaseGateway health",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(healthURL), nil, cmd)
		},
	}

	return cmd
}
