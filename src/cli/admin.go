package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/admin/v1/pdu"
	"github.com/urfave/cli"
)

func CreatePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PluginCreationRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().CreatePlugin(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("stdin: %v", err))
			return errs.Return()
		}

		do("stdin", data)
	} else {
		for _, file := range args {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				errs.append(fmt.Errorf("%s: %v", file, err))
				continue
			}

			do(file, data)
		}
	}

	return errs.Return()
}

func DeletePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) == 0 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	for _, pluginName := range args {
		resp, err := adminApi().DeletePluginByName(pluginName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			continue
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pluginName, resp.Error.Error))
			continue
		}
	}

	return errs.Return()
}

func RetrievePlugins(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source, namePattern string) {
		req := new(pdu.PluginsRetrieveRequest)
		req.NamePattern = namePattern

		retrieveResp, apiResp, err := adminApi().GetPlugins(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	if len(args) == 0 {
		do("all plugins", "")
	} else {
		for _, pluginName := range args {
			do(pluginName, pluginName)
		}
	}

	return errs.Return()
}

func UpdatePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PluginUpdateRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().UpdatePlugin(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("stdin: %v", err))
			return errs.Return()
		}

		do("stdin", data)
	} else {
		for _, file := range args {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				errs.append(fmt.Errorf("%s: %v", file, err))
				continue
			}

			do(file, data)
		}
	}

	return errs.Return()
}

func CreatePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PipelineCreationRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().CreatePipeline(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("stdin: %v", err))
			return errs.Return()
		}

		do("stdin", data)
	} else {
		for _, file := range args {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				errs.append(fmt.Errorf("%s: %v", file, err))
				continue
			}

			do(file, data)
		}
	}

	return errs.Return()
}

func DeletePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) == 0 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	for _, pipelineName := range args {
		resp, err := adminApi().DeletePipelineByName(pipelineName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, resp.Error.Error))
			continue
		}
	}

	return errs.Return()
}

func RetrievePipelines(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source, namePattern string) {
		req := new(pdu.PipelinesRetrieveRequest)
		req.NamePattern = namePattern

		retrieveResp, apiResp, err := adminApi().GetPipelines(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	if len(args) == 0 {
		do("all pipelines", "")
	} else {
		for _, pipelineName := range args {
			do(pipelineName, pipelineName)
		}
	}

	return errs.Return()
}

func UpdatePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PipelineUpdateRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().UpdatePipeline(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("stdin: %v", err))
			return errs.Return()
		}
		do("stdin", data)
	} else {
		for _, file := range args {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				errs.append(fmt.Errorf("%s: %v", file, err))
				continue
			}

			do(file, data)
		}
	}

	return errs.Return()
}

func RetrievePluginTypes(c *cli.Context) error {
	retrieveResp, apiResp, err := adminApi().GetPluginTypes()
	if err != nil {
		return err
	} else if apiResp.Error != nil {
		return fmt.Errorf("%s", apiResp.Error.Error)
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		return err
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)
	return nil
}

func RetrievePipelineTypes(c *cli.Context) error {
	retrieveResp, apiResp, err := adminApi().GetPipelineTypes()
	if err != nil {
		return err
	} else if apiResp.Error != nil {
		return fmt.Errorf("%s", apiResp.Error.Error)
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		return err
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)
	return nil
}
