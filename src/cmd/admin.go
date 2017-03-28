package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "admin"
	app.Usage = "gateway admin client"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
		{
			Name:  "plugin",
			Usage: "Plugin Subcommmand",
			Flags: []cli.Flag{},
			Subcommands: []cli.Command{
				{
					Name: "add",
					// TODO: add -f --force to cover current config of existed plugins
					Usage:  "Add one or more plugins",
					Action: pluginAdd,
				},
				{
					Name:   "rm",
					Usage:  "Remove one or more plugins",
					Action: pluginDelete,
				},
				{
					Name: "ls",
					// TODO: add --only-name -n
					// TODO: add --type -t
					Usage:  "List one or more plugins",
					Action: pluginGet,
				},
				{
					Name: "update",
					// TODO: add -f --force to add plugins which do not exist
					Usage:  "Update one or more plugins",
					Action: pluginUpdate,
				},
			},
		},
		{
			Name:  "pipeline",
			Usage: "Pipeline Subcommand",
			Flags: []cli.Flag{},
			Subcommands: []cli.Command{
				{
					Name: "add",
					// TODO: add -f --force to cover current config of existed pipelines
					Usage:  "Add one or more pipelines",
					Action: pipelineAdd,
				},
				{
					Name:   "rm",
					Usage:  "Remove one or more pipelines",
					Action: pipelineDelete,
				},
				{
					Name: "ls",
					// TODO: add -n --only-name
					Usage:  "List one or more pipelines",
					Action: pipelineGet,
				},
				{
					Name: "update",
					// TODO: add -f --force to add plugins which do not exist
					Usage:  "Update one or more pipelines",
					Action: pipelineUpdate,
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Print(err)
	}
}

const SERVER_ADDRESS = "http://127.0.0.1:9090"

var client = http.DefaultClient

func post(url, body string) (*http.Response, error) {
	return client.Post(SERVER_ADDRESS+url, "application/json", strings.NewReader(body))
}

func delete_(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, SERVER_ADDRESS+url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func get(url, body string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, SERVER_ADDRESS+url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func put(url, body string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, SERVER_ADDRESS+url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

type multipleErr struct {
	errs []error
}

func (e *multipleErr) append(err error) {
	e.errs = append(e.errs, err)
}

func (e *multipleErr) String() string {
	if e.errs == nil {
		return "<nil>"
	}

	var s string
	for _, err := range e.errs {
		s = fmt.Sprintf("%s%s\n", s, err.Error())
	}
	return s
}

func (e *multipleErr) Error() string {
	return e.String()
}

// supply the interface gap
func (e *multipleErr) Return() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e
}
