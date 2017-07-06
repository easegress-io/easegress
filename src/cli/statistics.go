package cli

import (
	"encoding/json"
	"fmt"

	"github.com/urfave/cli"
)

func RetrievePluginIndicatorNames(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	for _, pluginName := range args[1:] {
		retrieveResp, apiResp, err := statApi().GetPluginIndicatorNames(pipelineName, pluginName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, pluginName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, pluginName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, pluginName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetPluginIndicatorValue(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	} else if len(args) < 3 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	pluginName := args[1]

	for _, indicatorName := range args[2:] {
		value, apiResp, err := statApi().GetPluginIndicatorValue(pipelineName, pluginName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(
				fmt.Errorf("%s-%s-%s: %s",
					pipelineName, pluginName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetPluginIndicatorDesc(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	} else if len(args) < 3 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	pluginName := args[1]

	for _, indicatorName := range args[2:] {
		desc, apiResp, err := statApi().GetPluginIndicatorDesc(pipelineName, pluginName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %s",
				pipelineName, pluginName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func RetrievePipelineIndicatorNames(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	for _, pipelineName := range args {
		retrieveResp, apiResp, err := statApi().GetPipelineIndicatorNames(pipelineName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetPipelineIndicatorValue(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	for _, indicatorName := range args[1:] {
		value, apiResp, err := statApi().GetPipelineIndicatorValue(pipelineName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetPipelineIndicatorDesc(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	for _, indicatorName := range args[1:] {
		desc, apiResp, err := statApi().GetPipelineIndicatorDesc(pipelineName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func RetrieveTaskIndicatorNames(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	for _, pipelineName := range args {
		retrieveResp, apiResp, err := statApi().GetTaskIndicatorNames(pipelineName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetTaskIndicatorValue(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	for _, indicatorName := range args[1:] {
		value, apiResp, err := statApi().GetTaskIndicatorValue(pipelineName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func GetTaskIndicatorDesc(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	for _, indicatorName := range args[1:] {
		desc, apiResp, err := statApi().GetTaskIndicatorDesc(pipelineName, indicatorName)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}
