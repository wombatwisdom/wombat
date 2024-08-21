package main

import (
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
)

var listBinariesCommand = &cli.Command{
	Name:  "list",
	Usage: "List the available binaries",
	Action: func(context *cli.Context) error {
		bin, err := binaries.New()
		if err != nil {
			color.Red("failed to initialize: %v", err)
			return cli.Exit(err, 1)
		}

		l, err := bin.List()
		if err != nil {
			color.Red("failed to list binaries: %v", err)
			return cli.Exit(err, 1)
		}

		if len(l) == 0 {
			color.Red("no binaries found")
			return nil
		}

		for _, b := range l {
			if b == bin.Current() {
				color.Green("- %s", b)
			} else {
				color.White("- %s", b)
			}
		}

		return nil
	},
}
