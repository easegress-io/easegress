package main

import (
	"fmt"
	"os"
	"time"

	urfavecli "github.com/urfave/cli"

	"cli"
	"version"
)

func parseHost(c *urfavecli.Context) error {
	address := c.String("address")
	if len(address) != 0 {
		return cli.SetGatewayServerAddress(address)
	}
	return nil
}

func main() {
	urfavecli.BashCompletionFlag = urfavecli.BoolFlag{
		Name:   "Bash completion generation",
		Hidden: true,
	}

	app := urfavecli.NewApp()
	app.Name = "Ease Gateway command line interface"
	app.Usage = ""
	app.Version = fmt.Sprintf("release=%s, commit=%s, repo=%s", version.RELEASE, version.COMMIT, version.REPO)
	app.Compiled = time.Now()
	app.Copyright = "(c) 2017 MegaEase.com"
	app.Before = parseHost

	app.Flags = []urfavecli.Flag{
		urfavecli.StringFlag{
			Name:  "address",
			Usage: "Indicates gateway service instance address",
			Value: cli.GatewayServerAddress(),
		},
	}

	app.Commands = []urfavecli.Command{
		{
			Name:  "admin",
			Usage: "Administration interface",
			Subcommands: []urfavecli.Command{
				{
					Name:  "plugin",
					Usage: "Plugin administration interface",
					Flags: []urfavecli.Flag{},
					Subcommands: []urfavecli.Command{
						{
							Name: "add",
							// TODO: add -f --force to overwrite existing plugins
							Usage:  "Create one or more plugins",
							Action: cli.CreatePlugin,
						},
						{
							Name:   "rm",
							Usage:  "Delete one or more plugins",
							Action: cli.DeletePlugin,
						},
						{
							Name: "ls",
							// TODO: add --only-name -n
							// TODO: add --format (json, xml...)
							// TODO: add --type -t
							Usage:  "Retrieve plugins",
							Action: cli.RetrievePlugins,
						},
						{
							Name: "update",
							// TODO: add -f --force to add plugins which does not exist
							Usage:  "Update one or more plugins",
							Action: cli.UpdatePlugin,
						},
						{
							Name:   "types",
							Usage:  "Retrieve plugin types",
							Action: cli.RetrievePluginTypes,
						},
					},
				},
				{
					Name:  "pipeline",
					Usage: "Pipeline administration interface",
					Flags: []urfavecli.Flag{},
					Subcommands: []urfavecli.Command{
						{
							Name: "add",
							// TODO: add -f --force to overwrite existing pipelines
							Usage:  "Create one or more pipelines",
							Action: cli.CreatePipeline,
						},
						{
							Name:   "rm",
							Usage:  "Delete one or more pipelines",
							Action: cli.DeletePipeline,
						},
						{
							Name: "ls",
							// TODO: add -n --only-name
							// TODO: add --format (json, xml...)
							Usage:  "Retrieve pipelines",
							Action: cli.RetrievePipelines,
						},
						{
							Name: "update",
							// TODO: add -f --force to add pipeline which does not exist
							Usage:  "Update one or more pipelines",
							Action: cli.UpdatePipeline,
						},
						{
							Name:   "types",
							Usage:  "Retrieve pipeline types",
							Action: cli.RetrievePipelineTypes,
						},
					},
				},
			},
		},
		{
			Name:  "stat",
			Usage: "Statistics interface",
			Subcommands: []urfavecli.Command{
				{
					Name:  "plugin",
					Usage: "Plugin statistics interface",
					Flags: []urfavecli.Flag{},
					Subcommands: []urfavecli.Command{
						{
							Name: "ls",
							// TODO: add --format (json, xml...)
							Usage:  "Retrieve plugin indicator names",
							Action: cli.RetrievePluginIndicatorNames,
						},
						{
							Name:   "value",
							Usage:  "Get plugin indicator value",
							Action: cli.GetPluginIndicatorValue,
						},
						{
							Name:   "desc",
							Usage:  "Get plugin indicator description",
							Action: cli.GetPluginIndicatorDesc,
						},
					},
				},
				{
					Name:  "pipeline",
					Usage: "Pipeline statistics interface",
					Flags: []urfavecli.Flag{},
					Subcommands: []urfavecli.Command{
						{
							Name: "ls",
							// TODO: add --format (json, xml...)
							Usage:  "Retrieve pipeline indicator names",
							Action: cli.RetrievePipelineIndicatorNames,
						},
						{
							Name:   "value",
							Usage:  "Get pipeline indicator value",
							Action: cli.GetPipelineIndicatorValue,
						},
						{
							Name:   "desc",
							Usage:  "Get pipeline indicator description",
							Action: cli.GetPipelineIndicatorDesc,
						},
					},
				},
				{
					Name:  "task",
					Usage: "Task statistics interface",
					Flags: []urfavecli.Flag{},
					Subcommands: []urfavecli.Command{
						{
							Name: "ls",
							// TODO: add --format (json, xml...)
							Usage:  "Retrieve pipeline indicator names",
							Action: cli.RetrieveTaskIndicatorNames,
						},
						{
							Name:   "value",
							Usage:  "Get pipeline indicator value",
							Action: cli.GetTaskIndicatorValue,
						},
						{
							Name:   "desc",
							Usage:  "Get pipeline indicator description",
							Action: cli.GetTaskIndicatorDesc,
						},
					},
				},
			},
		},
		{
			Name:  "health",
			Usage: "Health Interface",
			Subcommands: []urfavecli.Command{
				{
					Name:   "check",
					Usage:  "Check health of Gateway",
					Action: cli.CheckHealth,
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Print(err)
	}
}
