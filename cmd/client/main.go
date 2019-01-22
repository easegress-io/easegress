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
  egctl api list

  # List member information.
  egctl member list

  # List plugin types.
  egctl plugin types

  # Create a plugin from a json file.
  egctl plugin create -f http_server.json

  # Create a plugin from stdout.
  cat http_server.json | egctl plugin create

  # Delete a plugin.
  egctl plugin delete <plugin name>

  # Get a plugin.
  egctl plugin get <plugin name>

  # List plugins.
  egctl plugin list

  # Update a plugin from a json file.
  egctl plugin update -f <http_server_2.json>

  # Update a plugin from stdout.
  echo http_server_2.json | egctl plugin update

  # Create a pipeline from a json file.
  egctl pipeline create -f <http_proxy_pipeline.json>

  # Create a pipeline from stdout.
  cat http_server.json | egctl pipeline create

  # Delete a pipeline.
  egctl pipeline delete <http_proxy_pipeline>

  # Get a pipeline.
  egctl pipeline get <http_proxy_pipeline>

  # List pipelines.
  egctl pipeline list

  # Update a pipeline from a json file.
  egctl pipeline update -f <http_proxy_pipeline_2.json>

  # Update a pipeline from stdout.
  echo <http_proxy_pipeline_2.json> | egctl pipeline update

  # Statistics pipeline.
  egctl stats
`

func main() {
	rootCmd := &cobra.Command{
		Use:        "egctl",
		Short:      "A command line admin tool for EaseGateway.",
		Example:    exampleUsage,
		SuggestFor: []string{"egctl"},
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
				command.ExitWithErrorf("unsupported shell %s, expecting bash or zsh", args[0])
			}
		},
		Args: cobra.ExactArgs(1),
	}

	rootCmd.AddCommand(
		command.APICmd(),
		command.PluginCmd(),
		command.PipelineCmd(),
		command.StatCmd(),
		command.MemberCmd(),
		completionCmd,
	)

	rootCmd.PersistentFlags().StringVar(&command.CommandlineGlobalFlags.Server,
		"server", "localhost:2381", "The address of the EaseGateway endpoint")

	err := rootCmd.Execute()
	if err != nil {
		command.ExitWithError(err)
	}
}
