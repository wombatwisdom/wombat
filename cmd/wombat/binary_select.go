package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
)

var selectBinaryCommand = &cli.Command{
	Name:      "select",
	Usage:     "Select the binary to use",
	Args:      true,
	ArgsUsage: "<name>",
	Action: func(context *cli.Context) error {
		if context.NArg() != 1 {
			color.Red("invalid number of arguments")
			return cli.Exit("invalid number of arguments", 1)
		}

		bin, err := binaries.New()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to initialize: %v", err), 1)
		}

		if err := bin.Select(context.Args().First()); err != nil {
			return cli.Exit(fmt.Sprintf("failed to select binary: %v", err), 1)
		}

		color.Green("%s selected", bin.Current())

		return nil
	},
}
