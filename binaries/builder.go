package binaries

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"text/template"
	"time"
)

//go:embed templates/go.mod.template
var goModTemplate string

//go:embed templates/main.go.template
var mainGoTemplate string

var templates = map[string]string{
	"main.go": mainGoTemplate,
	"go.mod":  goModTemplate,
}

type templateVars struct {
	GoVersion      string
	BenthosVersion string
	DateBuilt      string
	Packages       []string
}

type Builder interface {
	Build(ctx context.Context, spec Spec) error
}

func NewBuilder(targetDir string) (Builder, error) {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target dir: %w", err)
	}

	return &baseBuilder{
		targetDir: targetDir,
	}, nil
}

type baseBuilder struct {
	targetDir string
}

func (bb *baseBuilder) Build(ctx context.Context, spec Spec) error {
	logger := log.With().Str("benthos", spec.BenthosVersion).Str("name", spec.Name).Logger()

	// -- check if the goroot env var is set
	goExec := os.Getenv("GOROOT")
	if goExec == "" {
		return fmt.Errorf("GOROOT env var not set")
	}

	// -- create the temp workdir
	workDir, err := os.MkdirTemp("", "wombat-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			logger.Error().Err(err).Msg("failed to clean temp dir")
		}
	}()

	c := InDir(workDir, path.Join(goExec, "bin", "go"))
	gover, err := c.GoVersion(ctx)

	// -- build the template vars
	vars := templateVars{
		GoVersion:      gover,
		BenthosVersion: spec.BenthosVersion,
		DateBuilt:      time.Now().Format(time.DateOnly),
	}

	for _, lib := range spec.Libraries {
		for _, p := range lib.Packages {
			vars.Packages = append(vars.Packages, fmt.Sprintf("%s/%s", lib.Module, p))
		}
	}

	if err := bb.Generate(vars, workDir); err != nil {
		return fmt.Errorf("failed to generate module files: %w", err)
	}

	// -- add the modules
	for _, lib := range spec.Libraries {
		logger.Debug().Msgf("getting library %s@%s", lib.Module, lib.Version)
		if err := c.GoGet(ctx, fmt.Sprintf("%s@%s", lib.Module, lib.Version)); err != nil {
			return fmt.Errorf("failed to get library %s : %w", lib.Module, err)
		}
	}

	logger.Debug().Msg("pulling in module imports")
	if err := c.GoModTidy(ctx); err != nil {
		return fmt.Errorf("failed to tidy go modules: %w", err)
	}

	targetFilename := path.Join(bb.targetDir, spec.Name)

	logger.Info().Msgf("building %s", spec.Name)
	if err := c.GoBuild(ctx, runtime.GOOS, runtime.GOARCH, targetFilename); err != nil {
		return fmt.Errorf("failed to build %s: %w", spec.Name, err)
	}

	return nil
}

func (bb *baseBuilder) Generate(vars templateVars, dir string) error {
	// -- clean the directory if it exists
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clean temp dir: %w", err)
	}

	for f, tmpl := range templates {
		if err := os.MkdirAll(path.Dir(filepath.Join(dir, f)), 0755); err != nil {
			return fmt.Errorf("failed to create dir: %w", err)
		}

		outFile, err := os.Create(filepath.Join(dir, f))
		if err != nil {
			return fmt.Errorf("failed to create %v: %w", f, err)
		}
		outTemplate, err := template.New(f).Parse(tmpl)
		if err != nil {
			return fmt.Errorf("failed to initialise %v template: %w", f, err)
		}
		if err := outTemplate.Execute(outFile, vars); err != nil {
			return fmt.Errorf("failed to execute %v template: %w", f, err)
		}
		if err := outFile.Close(); err != nil {
			return fmt.Errorf("failed to close %v file: %w", f, err)
		}
	}

	return nil
}
