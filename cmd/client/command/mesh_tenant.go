package command

import (
	"errors"
	"github.com/spf13/cobra"
	"net/http"
)

func tenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "query and manager tenant",
	}
	cmd.AddCommand(createTenantCmd())
	cmd.AddCommand(updateTenantCmd())
	cmd.AddCommand(deleteTenantCmd())
	cmd.AddCommand(getTenantCmd())
	cmd.AddCommand(listTenantsCmd())
	return cmd
}

func createTenantCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an tenant from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshTenantsURL), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the tenant.")

	return cmd
}

func updateTenantCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an tenant from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshTenantURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the tenant.")

	return cmd
}

func deleteTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an tenant",
		Example: "egctl mesh tenant delete <tenant_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one tenant name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshTenantURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an tenant",
		Example: "egctl mesh tenant get <tenant_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one tenant name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshTenantURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listTenantsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all tenants",
		Example: "egctl mesh tenant list",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshTenantsURL), nil, cmd)
		},
	}

	return cmd
}
