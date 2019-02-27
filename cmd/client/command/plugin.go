package command

import (
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

func PluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "View and change plugins",
	}

	cmd.AddCommand(pluginTypeCmd())
	cmd.AddCommand(listPluginCmd())
	cmd.AddCommand(getPluginCmd())
	cmd.AddCommand(createPluginCmd())
	cmd.AddCommand(updatePluginCmd())
	cmd.AddCommand(deletePluginCmd())

	return cmd
}

func pluginTypeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "types",
		Short: "List available plugin types.",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(pluginTypesURL), nil, cmd)
		},
	}

	return cmd
}

func createPluginCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a plugin from a json file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			jsonText, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(pluginsURL), jsonText, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A json file specifying the plugin.")

	return cmd
}

func updatePluginCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a plugin from a json file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			jsonText, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(pluginURL, name), jsonText, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A json file specifying the plugin.")

	return cmd
}

func deletePluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a plugin",
		Example: "egctl plugin delete <plugin_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one plugin name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(pluginURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get plugin",
		Example: "egctl plugin get <plugin_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one plugin name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(pluginURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all plugins",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(pluginsURL), nil, cmd)
		},
	}

	return cmd
}
