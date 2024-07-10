package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"os"

	_ "github.com/redpanda-data/connect/public/bundle/free/v4"
	_ "github.com/wombatwisdom/wombat/public/components/nats"
)

func main() {
	app := initApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Failed to run app")
	}
}

func initApp() *cli.App {
	app := &cli.App{
		Name:        "docgen",
		Usage:       "Generate wombat documentation",
		Description: "Generate the documentation for wombat components",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "the directory to output the docs to",
				Required: true,
			},
		},
		Action: func(context *cli.Context) error {
			filename := fmt.Sprintf("%s/schema.json", context.Path("output"))
			color.Green("Generating docs to %s", filename)

			env := service.GlobalEnvironment()
			benv := bloblang.GlobalEnvironment()

			gen, err := NewJSONGenerator(context.Path("output"))
			if err != nil {
				color.Red("Failed to create generator: %v", err)
				return nil
			}

			if err := gen.Generate(env, benv); err != nil {
				color.Red("Failed to generate docs: %v", err)
			}

			color.Green("docs generated to %s", filename)
			return nil
		},
	}

	return app
}

type Generator interface {
	Generate(env *service.Environment, benv *bloblang.Environment) error
}
