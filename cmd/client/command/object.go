package command

import (
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

// ObjectCmd defines object command.
func ObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "object",
		Aliases: []string{"o", "obj"},
		Short:   "View and change objects",
	}

	cmd.AddCommand(objectKindsCmd())
	cmd.AddCommand(listObjectsCmd())
	cmd.AddCommand(getObjectCmd())
	cmd.AddCommand(createObjectCmd())
	cmd.AddCommand(updateObjectCmd())
	cmd.AddCommand(deleteObjectCmd())
	cmd.AddCommand(statusObjectCmd())

	return cmd
}

func objectKindsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kinds",
		Short: "List available object kinds.",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectKindsURL), nil, cmd)
		},
	}

	return cmd
}

func createObjectCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an object from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(objectsURL), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}

func updateObjectCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an object from a yaml file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(objectURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A yaml file specifying the object.")

	return cmd
}

func deleteObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an object",
		Example: "egctl object delete <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(objectURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get an object",
		Example: "egctl object get <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listObjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all objects",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectsURL), nil, cmd)
		},
	}

	return cmd
}

func statusObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Get status of an object",
		Example: "egctl object status <object_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one object name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(objectStatusURL, args[0]), nil, cmd)
		},
	}

	return cmd
}
