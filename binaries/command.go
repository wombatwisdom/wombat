package binaries

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func InDir(dir string, goexec string) *InDirCommand {
	return &InDirCommand{dir: dir, goexec: goexec}
}

type InDirCommand struct {
	goexec string
	dir    string
}

func (i *InDirCommand) GoVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, i.goexec, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	raw := string(out)
	return strings.TrimPrefix(strings.Split(raw, " ")[2], "go"), nil
}

func (i *InDirCommand) GoGet(ctx context.Context, url string) error {
	cmd := exec.CommandContext(ctx, i.goexec, "get", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = i.dir
	return cmd.Run()
}

func (i *InDirCommand) GoModTidy(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, i.goexec, "mod", "tidy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = i.dir
	return cmd.Run()
}

func (i *InDirCommand) GoBuild(ctx context.Context, goos string, goarch string, target string) error {
	cmd := exec.CommandContext(ctx, i.goexec, "build", "-o", target)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", goos),
		fmt.Sprintf("GOARCH=%s", goarch),
		//fmt.Sprintf("CGO_ENABLED=1"),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = i.dir

	return cmd.Run()
}
