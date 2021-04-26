package command

import (
	"errors"
	"github.com/spf13/cobra"
	"net/http"
)

// Service instance cmd
func serviceInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "query and manager service's instances",
	}

	cmd.AddCommand(getServiceInstanceCmd())
	cmd.AddCommand(deleteServiceInstanceCmd())
	cmd.AddCommand(listServiceInstancesCmd())
	return cmd
}

func deleteServiceInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service instance",
		Example: "egctl mesh service instance delete <service_name> <instance_id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires one service name and instance_id  to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceInstanceURL, args[0], args[1]), nil, cmd)
		},
	}

	return cmd
}

func getServiceInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service instance",
		Example: "egctl mesh service instance get <service_name> <instance_id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("requires one service name and instance_id to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceInstanceURL, args[0], args[1]), nil, cmd)
		},
	}

	return cmd
}

func listServiceInstancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all service instances",
		Example: "egctl mesh service instance list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceInstancesURL), nil, cmd)
		},
	}

	return cmd
}
