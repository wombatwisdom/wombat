package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
)

var currentBinaryCommand = &cli.Command{
	Name:  "current",
	Usage: "Show the currently selected binary",
	Action: func(context *cli.Context) error {
		bin, err := binaries.New()
		if err != nil {
			color.Red("failed to initialize: %v", err)
			return cli.Exit(err, 1)
		}

		if bin.Current() == "" {
			color.Red("no binary selected")
		} else {
			fmt.Println(bin.Current())
		}

		return nil
	},
}
