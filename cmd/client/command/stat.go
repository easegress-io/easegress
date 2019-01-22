package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

func StatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Dump EaseGateway runtime statistics",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(statsURL), nil, cmd)
		},
	}

	return cmd
}
