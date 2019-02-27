package command

import (
	"errors"
	"net/http"

	"github.com/spf13/cobra"
)

func PipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "View and change pipelines",
	}

	cmd.AddCommand(listPipelineCmd())
	cmd.AddCommand(getPipelineCmd())
	cmd.AddCommand(createPipelineCmd())
	cmd.AddCommand(updatePipelineCmd())
	cmd.AddCommand(deletePipelineCmd())

	return cmd
}

func createPipelineCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pipeline from a json file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, _ := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPost, makeURL(pipelinesURL), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A json file specifying the pipeline.")

	return cmd
}

func updatePipelineCmd() *cobra.Command {
	var specFile string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a pipeline from a json file or stdin",
		Run: func(cmd *cobra.Command, args []string) {
			buff, name := readFromFileOrStdin(specFile, cmd)
			handleRequest(http.MethodPut, makeURL(pipelineURL, name), buff, cmd)
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "A json file specifying the pipeline.")

	return cmd
}

func deletePipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a pipeline",
		Example: "egctl pipeline delete <pipeline_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one pipeline name to be deleted")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodDelete, makeURL(pipelineURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func getPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a pipeline",
		Example: "egctl pipeline get <pipeline_name>",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one pipeline name to be retrieved")
			}

			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(pipelineURL, args[0]), nil, cmd)
		},
	}

	return cmd
}

func listPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all pipelines",
		Run: func(cmd *cobra.Command, args []string) {
			handleRequest(http.MethodGet, makeURL(pipelinesURL), nil, cmd)
		},
	}

	return cmd
}
