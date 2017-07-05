package cli

import (
	"common"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/cluster/admin/v1/pdu"
	"github.com/urfave/cli"
	"strings"
)

func setLocalOperationSequence(group string, seq uint64) error {
	store, err := newRCJSONFileStore(rcFullPath)
	if err != nil {
		return err
	}

	rc, err := store.load()
	if err != nil {
		return err
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

	if localErr == nil && localSeq < serverSeq {
		return 0, fmt.Errorf("the configure of group %s on the server side has changed\n", group)
	}

	return serverSeq, nil
}

func ClusterCreatePlugin(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte) {
		req := new(pdu.PluginCreationClusterRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		req.TimeoutSec = uint16(timeout.Seconds())
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

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}
	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := time.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		expiredTime := time.Now().Sub(startTime)

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

		startTime = time.Now()
		do(file, seq, data)
		expiredTime = time.Now().Sub(startTime)

		setLocalOperationSequence(group, seq)

		if timeout <= expiredTime && i < len(args)-1 {
			errs.append(fmt.Errorf("timeout: skip to handle [%s]", strings.Join(args[i+1:], ", ")))
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
	timeout := time.Duration(timeoutSec) * time.Second
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(pluginName string, seq uint64) {
		req := new(pdu.ClusterOperationRequest)
		req.TimeoutSec = uint16(timeout.Seconds())
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
	for i, pluginName := range args {
		startTime := time.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			break
		}
		expiredTime := time.Now().Sub(startTime)
		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: no time to handle", args[i:]))
			break
		}
		timeout -= expiredTime

		seq++

		startTime = time.Now()
		do(pluginName, seq)
		setLocalOperationSequence(group, seq+1)
		expiredTime = time.Now().Sub(startTime)
		if timeout <= expiredTime && i < len(args)-1 {
			errs.append(fmt.Errorf("timeout: no time to handle: %s", args[i+1:]))
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
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
