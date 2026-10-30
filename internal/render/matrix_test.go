//go:build matrix

package render

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greylabss/greyprint/internal/plan"
)

func TestGeneratedProjectMatrix(t *testing.T) {
	root := t.TempDir()
	routers := []string{"stdlib", "chi", "gin", "echo", "fiber"}
	databases := []string{"none", "postgres", "mysql", "sqlite"}
	accesses := []string{"driver", "sqlc", "gorm"}
	count := 0
	for _, router := range routers {
		for _, database := range databases {
			for _, access := range accesses {
				if database == "none" && access != "driver" {
					continue
				}
				for _, redis := range []bool{false, true} {
					for _, greyship := range []bool{false, true} {
						o := plan.Options{Name: "matrix", Module: "example.com/matrix", Router: router, Database: database, Access: access, Redis: redis, Greyship: greyship}
						tree, err := Tree(o)
						if err != nil {
							t.Fatalf("render %s/%s/%s: %v", router, database, access, err)
						}
						dir := filepath.Join(root, router, database, access, boolName(redis), boolName(greyship))
						goFiles := []string{}
						for path, data := range tree {
							target := filepath.Join(dir, filepath.FromSlash(path))
							if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(target, data, 0644); err != nil {
								t.Fatal(err)
							}
							if strings.HasSuffix(path, ".go") {
								if bytes.Contains(data, []byte("//")) || bytes.Contains(data, []byte("/*")) {
									t.Fatalf("generated comment in %s", path)
								}
								goFiles = append(goFiles, target)
							}
						}
						runMatrixCommand(t, dir, "go", "mod", "tidy")
						formatted := runMatrixCommand(t, dir, "gofmt", append([]string{"-l"}, goFiles...)...)
						if strings.TrimSpace(formatted) != "" {
							t.Fatalf("unformatted Go in %s: %s", dir, formatted)
						}
						runMatrixCommand(t, dir, "go", "vet", "./...")
						runMatrixCommand(t, dir, "go", "build", "./...")
						count++
					}
				}
			}
		}
	}
	t.Logf("validated %d generated projects", count)
}

func boolName(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func runMatrixCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s in %s: %v\n%s", name, strings.Join(args, " "), dir, err, output)
	}
	return string(output)
}
