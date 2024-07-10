package main

import (
	"fmt"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/rs/zerolog/log"
	"os"
)

type mdGenerator struct {
	outputDir string
}

func NewMarkdownGenerator(outputDir string) (Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, nil
	}

	return &jsonGenerator{
		outputDir: outputDir,
	}, nil
}

func (j *mdGenerator) Generate(env *service.Environment, benv *bloblang.Environment) error {
	env.WalkCaches(newMarkdownWalker("cache", j.outputDir))
	env.WalkBuffers(newMarkdownWalker("buffer", j.outputDir))
	env.WalkInputs(newMarkdownWalker("input", j.outputDir))
	env.WalkOutputs(newMarkdownWalker("output", j.outputDir))
	env.WalkProcessors(newMarkdownWalker("processor", j.outputDir))
	env.WalkRateLimits(newMarkdownWalker("rate_limit", j.outputDir))
	env.WalkTracers(newMarkdownWalker("tracer", j.outputDir))
	env.WalkScanners(newMarkdownWalker("scanner", j.outputDir))
	env.WalkMetrics(newMarkdownWalker("metric", j.outputDir))

	bloblOut := fmt.Sprintf("%s/%s", j.outputDir, "bloblang")
	_ = os.MkdirAll(bloblOut, 0755)
	benv.WalkFunctions(newMarkdownFunctionWalker("functions", bloblOut))
	benv.WalkMethods(newMarkdownMethodWalker("methods", bloblOut))

	return nil
}

func newMarkdownWalker(kind string, outputDir string) func(string, *service.ConfigView) {
	// -- make sure the output dir exists
	baseName := fmt.Sprintf("%s/%s", outputDir, kind)
	_ = os.MkdirAll(baseName, 0755)

	return func(name string, config *service.ConfigView) {
		b, err := config.RenderDocs()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to format %s %s", kind, name)
			return
		}

		filename := fmt.Sprintf("%s/%s.md", baseName, name)

		if err := os.WriteFile(filename, b, 0644); err != nil {
			log.Error().Err(err).Msgf("Failed to write %s %s", kind, name)
		}

		log.Info().Msgf("written %s %s", kind, name)
	}
}

func newMarkdownFunctionWalker(kind string, outputDir string) func(string, *bloblang.FunctionView) {
	// -- make sure the output dir exists
	baseName := fmt.Sprintf("%s/%s", outputDir, kind)
	_ = os.MkdirAll(baseName, 0755)

	return func(name string, spec *bloblang.FunctionView) {
		b, err := spec.FormatJSON()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to format %s %s", kind, name)
			return
		}

		filename := fmt.Sprintf("%s/%s.json", baseName, name)

		if err := os.WriteFile(filename, b, 0644); err != nil {
			log.Error().Err(err).Msgf("Failed to write %s %s", kind, name)
		}

		log.Info().Msgf("written %s %s", kind, name)
	}
}

func newMarkdownMethodWalker(kind string, outputDir string) func(string, *bloblang.MethodView) {
	// -- make sure the output dir exists
	baseName := fmt.Sprintf("%s/%s", outputDir, kind)
	_ = os.MkdirAll(baseName, 0755)

	return func(name string, spec *bloblang.MethodView) {
		b, err := spec.FormatJSON()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to format %s %s", kind, name)
			return
		}

		filename := fmt.Sprintf("%s/%s.json", baseName, name)

		if err := os.WriteFile(filename, b, 0644); err != nil {
			log.Error().Err(err).Msgf("Failed to write %s %s", kind, name)
		}

		log.Info().Msgf("written %s %s", kind, name)
	}
}
