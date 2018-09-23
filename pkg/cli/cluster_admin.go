package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/cluster/admin/v1/pdu"
	"github.com/urfave/cli"
)

func setLocalOperationSequence(group string, seq uint64) error {
	store, err := newRCJSONFileStore(rcFullPath)
	if err != nil {
		return err
	}

	rc, err := store.load()
	if err != nil {
		rc = &runtimeConfig{
			Sequences: make(map[string]uint64),
		}
	}

	rc.Sequences[group] = seq
	err = store.save(rc)
	if err != nil {
		return err
	}

	return nil
}

func getLocalOperationSequence(group string) (uint64, error) {
	store, err := newRCJSONFileStore(rcFullPath)
	if err != nil {
		return 0, err
	}

	rc, err := store.load()
	if err != nil {
		return 0, err
	}

	return rc.Sequences[group], nil
}

func getServerOperationSequence(group string, timeoutSec uint16) (uint64, error) {
	req := new(pdu.ClusterOperationSeqRetrieveRequest)
	req.TimeoutSec = timeoutSec
	retrieveResp, apiResp, err := clusterAdminApi().GetMaxOperationSequence(group, req)
	if err != nil {
		return 0, err
	} else if apiResp.Error != nil {
		return 0, fmt.Errorf("%s", apiResp.Error.Error)
	}

	if retrieveResp.ClusterGroup != group {
		fmt.Printf("BUG: server returns wrong cluster group, required %s but get %s.\n",
			group, retrieveResp.ClusterGroup)
		return 0, fmt.Errorf("server returns wrong cluster group")
	}

	return retrieveResp.OperationSeq, nil
}

func getOperationSequence(group string, timeoutSec uint16) (uint64, error) {
	localSeq, localErr := getLocalOperationSequence(group)
	if localErr != nil {
		fmt.Printf("Warning: failed to get operation sequence from local config: %v\n", localErr)
	}

	serverSeq, serverErr := getServerOperationSequence(group, timeoutSec)
	if serverErr != nil {
		return 0, serverErr
	}

	if localErr != nil {
		return serverSeq, nil
	} else if localSeq < serverSeq {
		return 0, fmt.Errorf("The configure of group %s on the server side has changed "+
			"since your last operation: local sequence{default stored in ~/.easegatewayrc} "+
			"is less than cluster sequence (%d < %d), use adminc [plugin|pipeline] [ls|types] to update the local sequence.",
			group, localSeq, serverSeq)
	}

	return serverSeq, nil
}

func pullToLocalOperationSequence(group string, timeoutSec uint16) {
	seq, err := getServerOperationSequence(group, timeoutSec)
	if err != nil {
		fmt.Println("pull operaion sequnce to local failed: get server operation sequence failed:", err)
		return
	}
	err = setLocalOperationSequence(group, seq)
	if err != nil {
		fmt.Println("pull operaion sequnce to local failed: set local sequence failed:", err)
		return
	}
}

func ClusterCreatePlugin(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte, t uint16) {
		req := new(pdu.PluginCreationClusterRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq

		resp, err := clusterAdminApi().CreatePlugin(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}

	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			break
		}

		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(file, seq, data, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterDeletePlugin(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(pluginName string, seq uint64, t uint16) {
		req := new(pdu.ClusterOperationRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq

		resp, err := clusterAdminApi().DeletePluginByName(group, pluginName, req)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pluginName, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	timeout := time.Duration(timeoutSec) * time.Second

	for i, pluginName := range args {
		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			break
		}

		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(pluginName, seq, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq+1)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf(
					"timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterRetrievePlugins(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")
	page := c.Generic("page").(*common.Uint32Value).Get().(uint32)
	limit := c.Generic("limit").(*common.Uint32Value).Get().(uint32)

	errs := &multipleErr{}

	do := func(pluginName string, t uint16) {
		req := new(pdu.ClusterRetrieveRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		retrieveResp, apiResp, err := clusterAdminApi().GetPluginByName(group, pluginName, req)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pluginName, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	doAll := func(t uint16) {
		req := new(pdu.PluginsRetrieveClusterRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		req.Page = page
		req.Limit = limit
		retrieveResp, apiResp, err := clusterAdminApi().GetPlugins(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%v", err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s", apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(err)
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		doAll(uint16(timeout.Seconds()))
	} else {
		for i, pluginName := range args {
			startTime := common.Now()
			do(pluginName, uint16(timeout.Seconds()))
			expiredTime := common.Since(startTime)

			if timeout <= expiredTime {
				if i < len(args)-1 {
					errs.append(fmt.Errorf(
						"timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
				}
				break
			}
			timeout -= expiredTime
		}
	}

	pullToLocalOperationSequence(group, timeoutSec)

	return errs.Return()
}

func ClusterUpdatePlugin(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte, t uint16) {
		req := new(pdu.PluginUpdateClusterRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq

		resp, err := clusterAdminApi().UpdatePlugin(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}

	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			break
		}

		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(file, seq, data, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterCreatePipeline(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte, t uint16) {
		req := new(pdu.PipelineCreationClusterRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq

		resp, err := clusterAdminApi().CreatePipeline(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}

	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			break
		}

		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(file, seq, data, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterDeletePipeline(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(pipelineName string, seq uint64, t uint16) {
		req := new(pdu.ClusterOperationRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq
		resp, err := clusterAdminApi().DeletePipelineByName(group, pipelineName, req)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	timeout := time.Duration(timeoutSec) * time.Second

	for i, pipelineName := range args {
		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			break
		}
		expiredTime := common.Since(startTime)
		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(pipelineName, seq, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq+1)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf(
					"timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterRetrievePipelines(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")
	page := c.Generic("page").(*common.Uint32Value).Get().(uint32)
	limit := c.Generic("limit").(*common.Uint32Value).Get().(uint32)

	errs := &multipleErr{}

	do := func(pipelineName string, t uint16) {
		req := new(pdu.ClusterRetrieveRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		retrieveResp, apiResp, err := clusterAdminApi().GetPipelineByName(group, pipelineName, req)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	doAll := func(t uint16) {
		req := new(pdu.PipelinesRetrieveClusterRequest)
		req.TimeoutSec = t
		req.Consistent = consistent
		req.Page = page
		req.Limit = limit
		retrieveResp, apiResp, err := clusterAdminApi().GetPipelines(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%v", err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s", apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(err)
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		doAll(uint16(timeout.Seconds()))
	} else {
		for i, pipelineName := range args {
			startTime := common.Now()
			do(pipelineName, uint16(timeout.Seconds()))
			expiredTime := common.Since(startTime)

			if timeout <= expiredTime {
				if i < len(args)-1 {
					errs.append(fmt.Errorf(
						"timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
				}
				break
			}
			timeout -= expiredTime
		}
	}

	pullToLocalOperationSequence(group, timeoutSec)

	return errs.Return()
}

func ClusterUpdatePipeline(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte, t uint16) {
		req := new(pdu.PipelineUpdateClusterRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		req.TimeoutSec = t
		req.Consistent = consistent
		req.OperationSeq = seq

		resp, err := clusterAdminApi().UpdatePipeline(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}

	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := common.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			break
		}

		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i:], ", ")))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = common.Now()
		do(file, seq, data, uint16(timeout.Seconds()))
		expiredTime = common.Since(startTime)

		setLocalOperationSequence(group, seq)

		if timeout <= expiredTime {
			if i < len(args)-1 {
				errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
			}
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
}

func ClusterRetrievePluginTypes(c *cli.Context) error {
	errs := &multipleErr{}

	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	req := new(pdu.ClusterRetrieveRequest)
	req.TimeoutSec = timeoutSec
	req.Consistent = consistent
	retrieveResp, apiResp, err := clusterAdminApi().GetPluginTypes(group, req)
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)

	pullToLocalOperationSequence(group, timeoutSec)

	return errs.Return()
}

func ClusterRetrievePipelineTypes(c *cli.Context) error {
	errs := &multipleErr{}

	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	consistent := c.GlobalBool("consistent")

	req := new(pdu.ClusterRetrieveRequest)
	req.TimeoutSec = timeoutSec
	req.Consistent = consistent
	retrieveResp, apiResp, err := clusterAdminApi().GetPipelineTypes(group, req)
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)

	pullToLocalOperationSequence(group, timeoutSec)

	return errs.Return()
}
