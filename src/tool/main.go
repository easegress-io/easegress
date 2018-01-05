package main

import (
	"os"
	"path/filepath"

	"github.com/urfave/cli"

	"common"
	"tool/oplog"
)

func main() {
	app := cli.NewApp()
	app.Name = "Ease Gateway tool command line interface"
	app.Usage = ""
	app.Copyright = "(c) 2018 MegaEase.com"

	app.Commands = []cli.Command{
		oplogCmd,
	}

	app.Run(os.Args)
}

var oplogCmd = cli.Command{
	Name:  "oplog",
	Usage: "oplog interface",
	Subcommands: []cli.Command{
		{
			Name:  "retrieve",
			Usage: "Retrieve oplog operations from specified sequence",
			Flags: []cli.Flag{
				cli.Uint64Flag{
					Name:  "begin",
					Value: 1,
					Usage: "indicates begin sequence of retrieving oplog operations",
				},
				cli.Uint64Flag{
					Name:  "count",
					Value: 5,
					Usage: "indicates total count of retrieving oplog operations",
				},
				cli.StringFlag{
					Name:  "path",
					Value: filepath.Join(common.INVENTORY_HOME_DIR, "oplog"),
					Usage: "indicates oplog dir path",
				},
			},
			Action: oplog.RetrieveOpLog,
		},
	},
}
