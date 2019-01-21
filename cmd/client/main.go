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
  easegateway-client api list

  # List member information.
  easegateway-client member list

  # List plugin types.
  easegateway-client plugin types

  # Create a plugin from a json file.
  easegateway-client plugin create -f http_server.json

  # Create a plugin from stdout.
  cat http_server.json | easegateway-client plugin create

  # Delete a plugin.
  easegateway-client plugin delete http_server

  # Get a plugin.
  easegateway-client plugin get http_server

  # List plugins.
  easegateway-client plugin list

  # Update a plugin from a json file.
  easegateway-client plugin update -f http_server_2.json

  # Update a plugin from stdout.
  echo http_server_2.json | easegateway-client plugin update

  # Create a pipeline from a json file.
  easegateway-client pipeline create -f http_proxy_pipeline.json

  # Create a pipeline from stdout.
  cat http_server.json | easegateway-client pipeline create

  # Delete a pipeline.
  easegateway-client pipeline delete http_proxy_pipeline

  # Get a pipeline.
  easegateway-client pipeline get http_proxy_pipeline

  # List pipelines.
  easegateway-client pipeline list

  # Update a pipeline from a json file.
  easegateway-client pipeline update -f http_proxy_pipeline_2.json

  # Update a pipeline from stdout.
  echo http_proxy_pipeline_2.json | easegateway-client pipeline update

  # Statistics pipeline.
  easegateway-client stats
`

func main() {
	rootCmd := &cobra.Command{
		Use:        "easegateway-client",
		Short:      "A command line client for EaseStack.",
		Example:    exampleUsage,
		SuggestFor: []string{"easegateway-client"},
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
