package command

import (
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

// MemberCmd defines member command.
func MemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "View EaseGateway members",
	}

	cmd.AddCommand(listMemberCmd())
	cmd.AddCommand(purgeMemberCmd())
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

func purgeMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "purge <member name>",
		Short:   "Purge a EaseGateway member",
		Long:    "Purge a EaseGateway member. This command should be run after the easegateway node uninstalled",
		Example: "egctl member purge <member name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one member name to be deleted")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(memberURL, args[0]), nil, cmd)
		},
	}

	return cmd
}
