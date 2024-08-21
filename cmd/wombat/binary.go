package main

import "github.com/urfave/cli/v2"

var binaryCommand = &cli.Command{
	Name:    "binary",
	Aliases: []string{"bin"},
	Usage:   "Manage binaries",
	Subcommands: []*cli.Command{
		addBinaryCommand,
		currentBinaryCommand,
		deleteBinaryCommand,
		listBinariesCommand,
		selectBinaryCommand,
	},
}
