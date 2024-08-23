package main

import (
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/library"
)

var repoWisdomCommand = &cli.Command{
	Name:  "repo",
	Usage: "show information about the repository with the given name",
	Flags: []cli.Flag{
		wisdomDirFlag,
		wisdomUrlFlag,
	},
	Args:      true,
	ArgsUsage: "<repo-name>",
	Action: func(context *cli.Context) error {
		dir := getWisdomDir(context)
		url := getWisdomURL(context)
		lib, err := library.New(dir, url)
		if err != nil {
			color.Red("failed to initialize: %v", err)
			return cli.Exit(err, 1)
		}

		if context.NArg() != 1 {
			color.Red("invalid number of arguments")
			return cli.Exit("invalid number of arguments", 1)
		}

		repo, err := lib.Repository(context.Args().First())
		if err != nil {
			color.Red("failed to get repository: %v", err)
			return cli.Exit(err, 1)
		}

		color.Cyan(repo.Name)
		color.White(repo.Summary)
		color.White("URL: ", repo.SourceUrl)
		color.White("Versions:")
		for _, version := range repo.Versions {
			if version == repo.Latest {
				color.White("\t%s (latest)", version)
			} else {
				color.White("\t%s", version)
			}
		}

		return nil
	},
}
