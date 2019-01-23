package command

import (
	"net/http"

	"github.com/spf13/cobra"
)

func MemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "View EaseGateway members",
	}

	cmd.AddCommand(listMemberCmd())
	return cmd
}

func listMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List EaseGateway members",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(membersURL), nil, cmd)
		},
	}

	return cmd
}
