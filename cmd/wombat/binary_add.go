package main

import (
	_ "embed"
	"fmt"
	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"github.com/wombatwisdom/wombat/binaries"
	"github.com/wombatwisdom/wombat/binaries/presets"
	"io"
	"net/http"
	"os"
	"strings"
)

var addBinaryCommand = &cli.Command{
	Name:  "add",
	Usage: "Add a binary",
	Description: `
Add a binary to the list of available binaries by providing a link to a spec, either being a file or a URL.
`,
	Args:      true,
	ArgsUsage: "<path-to-spec-file>",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "select",
			Usage: "Select the binary after adding it",
		},
	},
	Action: func(context *cli.Context) error {
		if context.NArg() != 1 {
			color.Red("invalid number of arguments")
			return cli.Exit("invalid number of arguments", 1)
		}

		bin, err := binaries.New()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to initialize: %v", err), 1)
		}

		specFile := context.Args().First()
		var b []byte
		if strings.HasPrefix(specFile, "http") {
			resp, err := http.Get(specFile)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to fetch spec file: %v", err), 1)
			}

			if resp.StatusCode != http.StatusOK {
				return cli.Exit(fmt.Sprintf("failed to fetch spec file: %v", resp.Status), 1)
			}
			defer resp.Body.Close()

			b, err = io.ReadAll(resp.Body)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to read spec file: %v", err), 1)
			}

		} else if strings.HasPrefix(specFile, "preset:") {
			preset := strings.TrimPrefix(specFile, "preset:")
			b, err = presets.Get(preset)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to get preset: %v", err), 1)
			}

		} else {
			b, err = os.ReadFile(specFile)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to read spec file: %v", err), 1)
			}
		}

		var spec binaries.Spec
		if err := yaml.Unmarshal(b, &spec); err != nil {
			return cli.Exit(fmt.Sprintf("failed to unmarshal spec file: %v", err), 1)
		}

		if err := bin.Add(spec); err != nil {
			return cli.Exit(fmt.Sprintf("failed to add binary: %v", err), 1)
		}

		color.Green("binary %s added", spec.Name)

		if context.Bool("select") {
			if err := bin.Select(spec.Name); err != nil {
				return cli.Exit(fmt.Sprintf("failed to select binary: %v", err), 1)
			}
			color.Green("binary %s selected", spec.Name)
		}

		return nil
	},
}
