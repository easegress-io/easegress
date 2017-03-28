package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/urfave/cli"
)

func pluginAdd(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	postOnce := func(source, data string) {
		resp, err := post("/admin/v1/plugins", data)
		if err != nil {
			errs.append(fmt.Errorf("%s: %s", source, err))
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errs.append(fmt.Errorf("%s: read data from server failed: %s", source, err))
			return
		}

		if resp.StatusCode != 200 {
			errs.append(fmt.Errorf("%s: %d %s", source, resp.StatusCode, string(body)))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("/dev/stdin: %v", err))
			return errs.Return()
		}
		postOnce("/dev/stdin", string(data))
		return errs.Return()
	}

	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %s", file, err))
			continue
		}
		postOnce(file, string(data))
	}

	return errs.Return()
}

func pluginDelete(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) == 0 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	deleteOnce := func(source string) {
		resp, err := delete_("/admin/v1/plugins/" + source)
		if err != nil {
			errs.append(fmt.Errorf("%s: %s", source, err))
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errs.append(fmt.Errorf("%s: read data from server failed: %s", source, err))
			return
		}

		if resp.StatusCode != 200 {
			errs.append(fmt.Errorf("%s: %d %s", source, resp.StatusCode, string(body)))
			return
		}
	}

	for _, name := range args {
		deleteOnce(name)
	}

	return errs.Return()
}

func pluginGet(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	getAll := func() (string, error) {
		resp, err := get("/admin/v1/plugins", "{}")
		if err != nil {
			err = fmt.Errorf("%s", err)
			errs.append(err)
			return "", err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("read data from server failed: %s", err)
			errs.append(err)
			return "", err
		}

		if resp.StatusCode != 200 {
			err = fmt.Errorf("%d %s", resp.StatusCode, string(body))
			errs.append(err)
			return "", err
		}

		return string(body), nil
	}

	getOnce := func(source string) (string, error) {
		resp, err := get("/admin/v1/plugins/"+source, "{}")
		if err != nil {
			err = fmt.Errorf("%s: %s", source, err)
			errs.append(err)
			return "", err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("%s: read data from server failed: %s", source, err)
			errs.append(err)
			return "", err
		}

		if resp.StatusCode != 200 {
			err = fmt.Errorf("%s: %d %s", source, resp.StatusCode, string(body))
			errs.append(err)
			return "", err
		}

		return string(body), nil
	}

	if len(args) == 0 {
		body, err := getAll()
		if err == nil {
			fmt.Println(body)
		}
		// TODO: make it pretty
		return errs.Return()
	}

	for _, name := range args {
		body, err := getOnce(name)
		if err != nil {
			continue
		}
		fmt.Println(body)
	}

	return errs.Return()
}

func pluginUpdate(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	putOnce := func(source, data string) {
		resp, err := put("/admin/v1/plugins", data)
		if err != nil {
			errs.append(fmt.Errorf("%s: %s", source, err))
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errs.append(fmt.Errorf("%s: read data from server failed: %s", source, err))
			return
		}

		if resp.StatusCode != 200 {
			errs.append(fmt.Errorf("%s: %d %s", source, resp.StatusCode, string(body)))
			return
		}
	}

	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			errs.append(fmt.Errorf("/dev/stdin: %v", err))
			return errs.Return()
		}
		putOnce("/dev/stdin", string(data))
		return errs.Return()
	}

	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %s", file, err))
			continue
		}
		putOnce(file, string(data))
	}

	return errs.Return()
}
