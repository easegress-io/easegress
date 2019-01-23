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
  egctl plugin create -f <plugin_spec.json>

  # Create a plugin from stdout.
  cat <plugin_spec.json> | egctl plugin create

  # Delete a plugin.
  egctl plugin delete <plugin_name>

  # Get a plugin.
  egctl plugin get <plugin_name>

  # List plugins.
  egctl plugin list

  # Update a plugin from a json file.
  egctl plugin update -f <new_plugin_spec.json>

  # Update a plugin from stdout.
  echo <new_plugin_spec.json> | egctl plugin update

  # Create a pipeline from a json file.
  egctl pipeline create -f <pipeline_spec.json>

  # Create a pipeline from stdout.
  cat <pipeline_spec.json> | egctl pipeline create

  # Delete a pipeline.
  egctl pipeline delete <pipeline_name>

  # Get a pipeline.
  egctl pipeline get <pipeline_name>

  # List pipelines.
  egctl pipeline list

  # Update a pipeline from a json file.
  egctl pipeline update -f <new_pipeline_spec.json>

  # Update a pipeline from stdout.
  echo <new_pipeline_spec.json> | egctl pipeline update

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
