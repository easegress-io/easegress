package main

import (
	"os"

	"github.com/megaease/easegateway/cmd/client/command"
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var exampleUsage = `  # List APIs.
  egwctl api list

  # List member information.
  egwctl member list

  # List plugin types.
  egwctl plugin types

  # Create a plugin from a json file.
  egwctl plugin create -f http_server.json

  # Create a plugin from stdout.
  cat http_server.json | egwctl plugin create

  # Delete a plugin.
  egwctl plugin delete http_server

  # Get a plugin.
  egwctl plugin get http_server

  # List plugins.
  egwctl plugin list

  # Update a plugin from a json file.
  egwctl plugin update -f http_server_2.json

  # Update a plugin from stdout.
  echo http_server_2.json | egwctl plugin update

  # Create a pipeline from a json file.
  egwctl pipeline create -f http_proxy_pipeline.json

  # Create a pipeline from stdout.
  cat http_server.json | egwctl pipeline create

  # Delete a pipeline.
  egwctl pipeline delete http_proxy_pipeline

  # Get a pipeline.
  egwctl pipeline get http_proxy_pipeline

  # List pipelines.
  egwctl pipeline list

  # Update a pipeline from a json file.
  egwctl pipeline update -f http_proxy_pipeline_2.json

  # Update a pipeline from stdout.
  echo http_proxy_pipeline_2.json | egwctl pipeline update

  # Statistics pipeline.
  egwctl stats
`

func main() {
	rootCmd := &cobra.Command{
		Use:        "egwctl",
		Short:      "A command line client for EaseStack.",
		Example:    exampleUsage,
		SuggestFor: []string{"egwctl"},
	}

	completionCmd := &cobra.Command{
		Use:   "completion bash|zsh",
		Short: "Output shell completion code for the specified shell (bash or zsh)",
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				rootCmd.GenZshCompletion(os.Stdout)
			default:
				command.ExitWithErrorf("unsupported shell %s", args[0])
			}
		},
		Args: cobra.ExactArgs(1),
	}

	rootCmd.AddCommand(
		command.APICmd(),
		// command.PluginCmd(),
		// command.PipelineCmd(),
		// command.Stat(),
		// command.Member(),
		completionCmd,
	)

	rootCmd.PersistentFlags().StringVar(&command.CommandlineGlobalFlags.Server,
		"server", "localhost:2381", "The address of the EaseGateway endpoint")

	err := rootCmd.Execute()
	if err != nil {
		command.ExitWithError(err)
	}
}
