package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "View EaseGateway APIs",
	}

	cmd.AddCommand(listAPICmd())
	return cmd
}

func listAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List EaseGateway APIs",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(apiURL), nil, cmd)
		},
	}

	return cmd
}
