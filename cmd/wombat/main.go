package main

import (
	"context"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
	"golang.org/x/exp/slices"
	"os"
)

var passthroughCommands = []string{"echo", "lint", "run", "streams", "list", "create", "test", "template", "blobl"}

func main() {
	args := os.Args[1:]

	// -- if the command is a passthrough command, we will run the binary passing through all cli arguments
	if len(args) >= 1 && slices.Contains(passthroughCommands, args[0]) {
		bin, err := binaries.New()
		if err != nil {
			color.Red("failed to initialize: %v", err)
			os.Exit(1)
		}

		// -- check if a binary is selected
		if !bin.Exists(bin.Current()) {
			color.Red("no binary selected")
			os.Exit(1)
		}

		// -- run the binary
		err = bin.Run(context.Background(), bin.Current(), os.Args[1:]...) // -- pass through to the binary
		if err != nil {
			color.Red("failed to run binary: %v", err)
			os.Exit(1)
		}

		return
	}

	// in all other cases, we will run the cli
	app := initApp()
	if err := app.Run(os.Args); err != nil {
		color.Red("failed to run app: %v", err)
	}
}

func initApp() *cli.App {
	app := &cli.App{
		Name:  "wombat",
		Usage: "Stream processing for the modern world",
		Description: `
Either run Wombat as a stream processor or choose a command:

 wombat list inputs
 wombat create kafka//file > ./config.yaml
 wombat -c ./config.yaml
 wombat -r "./production/*.yaml" -c ./config.yaml
`,
		Commands: []*cli.Command{
			binaryCommand,
			wisdomCommand,
			{
				Name:  "echo",
				Usage: "Parse a config file and echo back a normalised version",
			},
			{
				Name:  "lint",
				Usage: "Parse Wombat configs and report any linting errors",
			},
			{
				Name:  "streams",
				Usage: "Run Wombat in streams mode",
			},
			{
				Name:  "list",
				Usage: "List all Wombat component types",
			},
			{
				Name:  "create",
				Usage: "Create a new Wombat config",
			},
			{
				Name:  "test",
				Usage: "Execute Wombat unit tests",
			},
			{
				Name:  "template",
				Usage: "Interact and generate Wombat templates",
			},
			{
				Name:  "blobl",
				Usage: "Execute a Wombat mapping on documents consumed via stdin",
			},
		}}

	return app
}
