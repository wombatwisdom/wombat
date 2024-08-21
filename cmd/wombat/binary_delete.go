package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
)

var deleteBinaryCommand = &cli.Command{
	Name:      "delete",
	Aliases:   []string{"del"},
	Usage:     "Delete a binary",
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

		if err := bin.Delete(context.Args().First()); err != nil {
			return cli.Exit(fmt.Sprintf("failed to delete binary: %v", err), 1)
		}

		color.Green("%s deleted", bin.Current())

		return nil
	},
}
