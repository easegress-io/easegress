package command

import (
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

func serviceCanaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "canary",
		Short: "query and manager service's canary rule",
	}

	cmd.AddCommand(createServiceCanaryCmd())
	cmd.AddCommand(updateServiceCanaryCmd())
	cmd.AddCommand(getServiceCanaryCmd())
	cmd.AddCommand(deleteServiceCanaryCmd())
	return cmd
}

func createServiceCanaryCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service canary from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceCanaryURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service canary.")

	return cmd
}

func updateServiceCanaryCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service canary from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceCanaryURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service canary.")

	return cmd
}

func deleteServiceCanaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service canary",
		Example: "egctl mesh service canary delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceCanaryURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceCanaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service canary",
		Example: "egctl mesh service canary get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceCanaryURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func serviceResilienceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resilience",
		Short: "query and manager service's resilience rule",
	}

	cmd.AddCommand(createServiceResilienceCmd())
	cmd.AddCommand(updateServiceResilienceCmd())
	cmd.AddCommand(getServiceResilienceCmd())
	cmd.AddCommand(deleteServiceResilienceCmd())
	return cmd
}

func createServiceResilienceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service resilience from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceResilienceURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service resilience.")

	return cmd
}

func updateServiceResilienceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service resilience from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceResilienceURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service resilience.")

	return cmd
}

func deleteServiceResilienceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service resilience",
		Example: "egctl mesh service resilience delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceResilienceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceResilienceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service resilience",
		Example: "egctl mesh service resilience get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceResilienceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func serviceLoadbalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loadbalance",
		Short: "query and manager service's loadbalance rule",
	}

	cmd.AddCommand(createServiceLoadbalanceCmd())
	cmd.AddCommand(updateServiceLoadbalanceCmd())
	cmd.AddCommand(getServiceLoadbalanceCmd())
	cmd.AddCommand(deleteServiceLoadbalanceCmd())
	return cmd
}

func createServiceLoadbalanceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service loadbalance from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceLoadBalanceURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service loadbalance.")

	return cmd
}

func updateServiceLoadbalanceCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service loadbalance from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceLoadBalanceURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service loadbalance.")

	return cmd
}

func deleteServiceLoadbalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service loadbalance",
		Example: "egctl mesh service loadbalance delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceLoadBalanceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceLoadbalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service loadbalance",
		Example: "egctl mesh service loadbalance get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceLoadBalanceURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func serviceOutputserverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outputserver",
		Short: "query and manager service's outputserver",
	}

	cmd.AddCommand(createServiceOutputserverCmd())
	cmd.AddCommand(updateServiceOutputserverCmd())
	cmd.AddCommand(getServiceOutputserverCmd())
	cmd.AddCommand(deleteServiceOutputserverCmd())
	return cmd
}

func createServiceOutputserverCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service outputserver from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceOutputServerURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service outputserver.")

	return cmd
}

func updateServiceOutputserverCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service outputserver from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceOutputServerURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service outputserver.")

	return cmd
}

func deleteServiceOutputserverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service outputserver",
		Example: "egctl mesh service outputserver delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceOutputServerURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceOutputserverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service outputserver",
		Example: "egctl mesh service outputserver get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceOutputServerURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func serviceTracingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tracings",
		Short: "query and manager service's tracings",
	}

	cmd.AddCommand(createServiceTracingsCmd())
	cmd.AddCommand(updateServiceTracingsCmd())
	cmd.AddCommand(getServiceTracingsCmd())
	cmd.AddCommand(deleteServiceTracingsCmd())
	return cmd
}

func createServiceTracingsCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service tracings from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceTracingsURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service tracings.")

	return cmd
}

func updateServiceTracingsCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service tracings from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceTracingsURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service tracings.")

	return cmd
}

func deleteServiceTracingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service tracings",
		Example: "egctl mesh service tracing delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceTracingsURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceTracingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service tracings",
		Example: "egctl mesh service tracing get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceTracingsURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

//  Service metric cmd
func serviceMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "query and manager service's metric",
	}

	cmd.AddCommand(createServiceMetricsCmd())
	cmd.AddCommand(updateServiceMetricsCmd())
	cmd.AddCommand(getServiceMetricsCmd())
	cmd.AddCommand(deleteServiceMetricsCmd())
	return cmd
}

func createServiceMetricsCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an service metrics from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be create")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(MeshServiceMetricsURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service metrics.")

	return cmd
}

func updateServiceMetricsCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an service metrics from a yaml file or stdin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be updated")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(MeshServiceMetricsURL, args[0]), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the service metrics.")

	return cmd
}

func deleteServiceMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an service metrics",
		Example: "egctl mesh service metric delete <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(MeshServiceMetricsURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getServiceMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an service metrics",
		Example: "egctl mesh service metric get <service_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one service name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(MeshServiceMetricsURL, args[0]), nil, cmd)
		},
	}

	return cmd
}
