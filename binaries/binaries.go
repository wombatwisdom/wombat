package binaries

import (
	"context"
	"os"
	"os/exec"
	"path"
)

type Spec struct {
	Name           string        `json:"name"`
	BenthosVersion string        `json:"benthos_version"`
	Libraries      []LibrarySpec `json:"libraries"`
}

type LibrarySpec struct {
	Module   string   `json:"module"`
	Version  string   `json:"version"`
	Packages []string `json:"packages"`
}

type Binaries interface {
	Run(ctx context.Context, name string, args ...string) error
	Add(spec Spec) error
	Select(name string) error
	Current() string
	Exists(name string) bool
	List() ([]string, error)
	Delete(name string) error
}

func New() (Binaries, error) {
	dir := os.ExpandEnv("${HOME}/.wombat")

	// -- make sure the dir exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	binDir := path.Join(dir, "binaries")

	builder, err := NewBuilder(binDir)
	if err != nil {
		return nil, err
	}

	return &binaries{dir: dir, binariesDir: binDir, builder: builder}, nil
}

type binaries struct {
	dir         string
	binariesDir string
	builder     Builder
}

func (b *binaries) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.Command(path.Join(b.binariesDir, name), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *binaries) Add(spec Spec) error {
	return b.builder.Build(context.Background(), spec)
}

func (b *binaries) Select(name string) error {
	if !b.Exists(name) {
		return os.ErrNotExist
	}

	return os.WriteFile(path.Join(b.dir, "current"), []byte(name), 0644)
}

func (b *binaries) Current() string {
	if _, err := os.Stat(path.Join(b.dir, "current")); err != nil {
		return ""
	}

	cur, err := os.ReadFile(path.Join(b.dir, "current"))
	if err != nil {
		return ""
	}

	return string(cur)
}

func (b *binaries) Exists(name string) bool {
	// -- check if the binary exists
	if _, err := os.Stat(path.Join(b.binariesDir, name)); err != nil {
		return false
	}

	return true
}

func (b *binaries) List() ([]string, error) {
	entries, err := os.ReadDir(b.binariesDir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names, nil
}

func (b *binaries) Delete(name string) error {
	if !b.Exists(name) {
		return os.ErrNotExist
	}

	return os.RemoveAll(path.Join(b.binariesDir, name))
}
