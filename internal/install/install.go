package install

import (
	"context"
	"fmt"
	"os/exec"
)

type Step struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func Run(ctx context.Context, dir string, noInstall, noGit bool) []Step {
	steps := []Step{}
	if !noInstall {
		steps = append(steps, run(ctx, dir, "go mod tidy", "go", "mod", "tidy"))
	}
	if !noGit {
		steps = append(steps, run(ctx, dir, "git init", "git", "init"))
	}
	return steps
}

func run(ctx context.Context, dir, name, command string, args ...string) Step {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return Step{Name: name, Error: fmt.Sprintf("%s failed: %v; rerun it by hand from %s", name, err, dir)}
	}
	return Step{Name: name, OK: true}
}
