package command

import (
	"errors"
	"github.com/spf13/cobra"
	"net/http"

	mesh "github.com/megaease/easegateway/pkg/object/meshcontroller/master"
)

func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "query and manager service",
	}

	cmd.AddCommand(createServiceCmd())
	cmd.AddCommand(updateServiceCmd())
	cmd.AddCommand(deleteServiceCmd())
	cmd.AddCommand(getServiceCmd())
	cmd.AddCommand(listServicesCmd())
	cmd.AddCommand(serviceCanaryCmd())
	cmd.AddCommand(serviceResilienceCmd())
	cmd.AddCommand(serviceLoadbalanceCmd())
	cmd.AddCommand(serviceOutputserverCmd())
	cmd.AddCommand(serviceTracingCmd())
	cmd.AddCommand(serviceMetricCmd())
	cmd.AddCommand(serviceInstanceCmd())
	return cmd
}

func createServiceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(mesh.MeshServicePath), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service.")

	return cmd
}

func updateServiceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(mesh.MeshServicePath, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service.")

	return cmd
}

func deleteServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service",
		Example: "egctl mesh service delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(mesh.MeshServicePath, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service",
		Example: "egctl mesh service get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(mesh.MeshServicePath, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listServicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all services",
		Example: "egctl mesh service list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(mesh.MeshServicePrefix), nil, cmd)
		},
	}

	return cmd
}
