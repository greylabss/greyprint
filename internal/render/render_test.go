package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greylabss/greyprint/internal/plan"
)

var updateGolden = flag.Bool("update", false, "update generated project golden files")

func TestTreeContainsExpectedFiles(t *testing.T) {
	o := plan.Options{Name: "sample", Module: "example.com/sample", Router: "stdlib", Database: "none", Access: "driver", Greyship: true}
	tree, err := Tree(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"cmd/api/main.go", "internal/config/config.go", "internal/routes/routes.go", "internal/routes/health.go", "go.mod", "README.md"} {
		if _, ok := tree[path]; !ok {
			t.Errorf("missing %s", path)
		}
	}
	if _, ok := tree["cmd/migrate/main.go"]; ok {
		t.Error("migrate command generated without a database")
	}
}

func TestTreeOptionFilesAndNoGeneratedGoComments(t *testing.T) {
	o := plan.Options{Name: "sample", Module: "example.com/sample", Router: "fiber", Database: "sqlite", Access: "driver", Redis: true, Greyship: true}
	tree, err := Tree(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"internal/database/database.go", "internal/cache/cache.go", "migrations/00001_init.sql", "cmd/migrate/main.go"} {
		if _, ok := tree[path]; !ok {
			t.Errorf("missing %s", path)
		}
	}
	for path, content := range tree {
		if strings.HasSuffix(path, ".go") && (bytes.Contains(content, []byte("//")) || bytes.Contains(content, []byte("/*"))) {
			t.Errorf("generated comments in %s", path)
		}
	}
}

func TestRouterTreesAreDeterministic(t *testing.T) {
	for _, router := range []string{"stdlib", "chi", "gin", "echo", "fiber"} {
		t.Run(router, func(t *testing.T) {
			o := plan.Options{Name: "sample", Module: "example.com/sample", Router: router, Database: "none", Access: "driver"}
			first, err := Tree(o)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Tree(o)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(Paths(first), "\n") != strings.Join(Paths(second), "\n") {
				t.Fatal("file paths changed")
			}
			for _, path := range Paths(first) {
				if !bytes.Equal(first[path], second[path]) {
					t.Fatalf("%s changed", path)
				}
			}
		})
	}
}

func TestGoldenTrees(t *testing.T) {
	cases := map[string]plan.Options{
		"default": {Name: "sample", Module: "example.com/sample", Router: "stdlib", Database: "none", Access: "driver", Greyship: true},
		"full":    {Name: "sample", Module: "example.com/sample", Router: "fiber", Database: "sqlite", Access: "gorm", Redis: true, Greyship: true},
	}
	root := filepath.Join("..", "..", "testdata", "golden")
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := Tree(options)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(root, name)
			if *updateGolden {
				if err := os.RemoveAll(base); err != nil {
					t.Fatal(err)
				}
				for path, data := range actual {
					target := filepath.Join(base, filepath.FromSlash(path))
					if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				return
			}
			files := map[string][]byte{}
			if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				relative, err := filepath.Rel(base, path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(relative)] = data
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(files) != len(actual) {
				t.Fatalf("golden has %d files, generated %d", len(files), len(actual))
			}
			for path, want := range files {
				if !bytes.Equal(actual[path], want) {
					t.Errorf("%s differs from golden", path)
				}
			}
		})
	}
}
