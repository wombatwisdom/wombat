package main

import (
	"fmt"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/rs/zerolog/log"
	"os"
)

type jsonGenerator struct {
	outputDir string
}

func NewJSONGenerator(outputDir string) (Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, nil
	}

	return &jsonGenerator{
		outputDir: outputDir,
	}, nil
}

func (j *jsonGenerator) Generate(env *service.Environment, benv *bloblang.Environment) error {
	env.WalkCaches(newWalker("cache", j.outputDir))
	env.WalkBuffers(newWalker("buffer", j.outputDir))
	env.WalkInputs(newWalker("input", j.outputDir))
	env.WalkOutputs(newWalker("output", j.outputDir))
	env.WalkProcessors(newWalker("processor", j.outputDir))
	env.WalkRateLimits(newWalker("rate_limit", j.outputDir))
	env.WalkTracers(newWalker("tracer", j.outputDir))
	env.WalkScanners(newWalker("scanner", j.outputDir))
	env.WalkMetrics(newWalker("metric", j.outputDir))

	bloblOut := fmt.Sprintf("%s/%s", j.outputDir, "bloblang")
	_ = os.MkdirAll(bloblOut, 0755)
	benv.WalkFunctions(newFunctionWalker("functions", bloblOut))
	benv.WalkMethods(newMethodWalker("methods", bloblOut))

	return nil
}

func newWalker(kind string, outputDir string) func(string, *service.ConfigView) {
	// -- make sure the output dir exists
	baseName := fmt.Sprintf("%s/%s", outputDir, kind)
	_ = os.MkdirAll(baseName, 0755)

	return func(name string, config *service.ConfigView) {
		b, err := config.FormatJSON()
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

func newFunctionWalker(kind string, outputDir string) func(string, *bloblang.FunctionView) {
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

func newMethodWalker(kind string, outputDir string) func(string, *bloblang.MethodView) {
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
