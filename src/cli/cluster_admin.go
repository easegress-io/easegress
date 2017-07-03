package cli

import (
	"time"

	"github.com/urfave/cli"
)

func getLocalOperationSequence(group string) (uint64, error) {
	return 0, nil
}

func queryOperationSequnce(group string, timeout time.Duration) (uint64, error) {
	return 0, nil
}

func ClusterCreatePlugin(c *cli.Context) error {
	return nil
}

func ClusterDeletePlugin(c *cli.Context) error {
	return nil
}

func ClusterRetrievePlugins(c *cli.Context) error {
	return nil
}

func ClusterUpdatePlugin(c *cli.Context) error {
	return nil
}

func ClusterCreatePipeline(c *cli.Context) error {
	return nil
}

func ClusterDeletePipeline(c *cli.Context) error {
	return nil
}

func ClusterRetrievePipelines(c *cli.Context) error {
	return nil
}

func ClusterUpdatePipeline(c *cli.Context) error {
	return nil
}

func ClusterRetrievePluginTypes(c *cli.Context) error {
	return nil
}

func ClusterRetrievePipelineTypes(c *cli.Context) error {
	return nil
}
