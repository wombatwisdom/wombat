package main

import (
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/library"
)

var updateWisdomCommand = &cli.Command{
	Name:  "update",
	Usage: "Update the local package cache",
	Flags: []cli.Flag{
		wisdomDirFlag,
		wisdomUrlFlag,
	},
	Action: func(context *cli.Context) error {
		dir := getWisdomDir(context)
		url := getWisdomURL(context)
		lib, err := library.New(dir, url)
		if err != nil {
			color.Red("failed to initialize: %v", err)
			return cli.Exit(err, 1)
		}

		if err := lib.Update(); err != nil {
			color.Red("failed to update: %v", err)
			return cli.Exit(err, 1)
		}

		color.Green("wisdom updated")

		return nil
	},
}
