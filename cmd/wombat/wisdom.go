package main

import "github.com/urfave/cli/v2"

var wisdomCommand = &cli.Command{
	Name:  "wisdom",
	Usage: "Access the wisdom, home of the wombats",
	Subcommands: []*cli.Command{
		updateWisdomCommand,
		searchWisdomCommand,
		repoWisdomCommand,
	},
}

var wisdomDirFlag = &cli.StringFlag{
	Name:        "dir",
	Usage:       "Local directory containing the wisdom cache",
	DefaultText: "$HOME/.wombat/wisdom",
}

var wisdomUrlFlag = &cli.StringFlag{
	Name:        "url",
	Usage:       "URL of the online wisdom library",
	DefaultText: "https://wisdom.wombat.dev/library.tgz",
}

func getWisdomDir(context *cli.Context) string {
	res := context.String("dir")

	if res == "" {
		res = "$HOME/.wombat/wisdom"
	}

	return res
}

func getWisdomURL(context *cli.Context) string {
	res := context.String("url")

	if res == "" {
		res = "https://wisdom.wombat.dev/library.tgz"
	}

	return res
}
