package main

import (
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/library"
	"strings"
)

var searchWisdomCommand = &cli.Command{
	Name:  "search",
	Usage: "Search for packages in the wisdom",
	Flags: []cli.Flag{
		wisdomDirFlag,
		wisdomUrlFlag,
		&cli.IntFlag{
			Name:        "offset",
			Aliases:     []string{"o"},
			Usage:       "Offset for the search results",
			DefaultText: "0",
		},
		&cli.IntFlag{
			Name:        "size",
			Aliases:     []string{"s"},
			Usage:       "Size of the search results",
			DefaultText: "25",
		},
	},
	Args:      true,
	ArgsUsage: "<query>",
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

		query := context.Args().First()

		offset := 0
		if context.IsSet("offset") {
			offset = context.Int("offset")
		}

		size := 25
		if context.IsSet("size") {
			size = context.Int("size")
		}

		res, err := lib.Search(query, offset, size)
		if err != nil {
			color.Red("failed to search: %v", err)
			return cli.Exit(err, 1)
		}

		color.Green("Found %d hits in %s", res.Total, res.Took)

		for idx, hit := range res.Hits {
			if idx > 0 {
				color.White("----------")
			}

			color.Cyan(strings.Join([]string{hit.Repository, hit.Version, hit.Package, hit.Name, hit.Kind}, "/"))
			color.White(hit.Summary)
			color.White("Status: ", hit.Status)
			color.White("License: %s", hit.License)
		}

		return nil
	},
}
