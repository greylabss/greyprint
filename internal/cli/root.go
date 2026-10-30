package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/greylabss/greyprint/internal/install"
	"github.com/greylabss/greyprint/internal/plan"
	"github.com/greylabss/greyprint/internal/render"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type result struct {
	Name    string         `json:"name"`
	Dir     string         `json:"dir"`
	Module  string         `json:"module"`
	Options plan.Options   `json:"options"`
	Files   []string       `json:"files"`
	Steps   []install.Step `json:"steps"`
}

var version = "dev"

func New() *cobra.Command {
	root := &cobra.Command{Use: "greyprint", Short: "Scaffold a minimal Go backend", Version: version, SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newCommand())
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the Greyprint version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "greyprint "+version)
		return err
	}})
	return root
}

func newCommand() *cobra.Command {
	var module, router, db, access, dir string
	var redis, greyship, yes, noInstall, noGit, dryRun, jsonOutput bool
	cmd := &cobra.Command{Use: "new [name]", Short: "Create a Go backend", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		o := plan.Options{Router: "stdlib", Database: "none", Access: "driver", Greyship: true}
		if name != "" {
			o.Name = name
		}
		flags := cmd.Flags()
		if flags.Changed("module") {
			o.Module = module
		}
		if flags.Changed("router") {
			o.Router = router
		}
		if flags.Changed("db") {
			o.Database = db
		}
		if flags.Changed("access") {
			o.Access = access
		}
		if flags.Changed("redis") {
			o.Redis = redis
		}
		if flags.Changed("greyship") {
			o.Greyship = greyship
		}
		if jsonOutput {
			yes = true
		}
		if !yes && !jsonOutput {
			if err := prompt(&o, cmd, flags); err != nil {
				return err
			}
		}
		if o.Name == "" {
			return errors.New("project name is required when prompts are disabled")
		}
		if o.Module == "" {
			o.Module = defaultModule(o.Name)
		}
		if dir == "" {
			dir = filepath.Join(".", o.Name)
		}
		o.Dir = dir
		if err := o.Validate(); err != nil {
			return err
		}
		tree, err := render.Tree(o)
		if err != nil {
			return err
		}
		paths := render.Paths(tree)
		if dryRun {
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result{Name: o.Name, Dir: dir, Module: o.Module, Options: o, Files: paths, Steps: []install.Step{}})
			}
			for _, path := range paths {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), filepath.ToSlash(path)); err != nil {
					return err
				}
			}
			return nil
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if info, err := os.Stat(absDir); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("output path %s is not a directory", absDir)
			}
			entries, err := os.ReadDir(absDir)
			if err != nil {
				return err
			}
			if len(entries) > 0 {
				return fmt.Errorf("output directory %s must be empty", absDir)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		for _, path := range paths {
			target := filepath.Join(absDir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(target, tree[path], 0644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
		steps := install.Run(cmd.Context(), absDir, noInstall, noGit)
		out := result{Name: o.Name, Dir: absDir, Module: o.Module, Options: o, Files: paths, Steps: steps}
		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "✓ Created %s\n\n  cd %s\n  cp .env.example .env\n  make run\n", o.Name, dir); err != nil {
			return err
		}
		if o.Greyship {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\n  Deploy:  grey deploy"); err != nil {
				return err
			}
		}
		for _, step := range steps {
			if !step.OK {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), step.Error); err != nil {
					return err
				}
			}
		}
		if noInstall {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\n  Run go mod tidy before building."); err != nil {
				return err
			}
		}
		return nil
	}}
	f := cmd.Flags()
	f.StringVar(&module, "module", "", "Go module path")
	f.StringVar(&router, "router", "stdlib", "HTTP router: stdlib, chi, gin, echo, fiber")
	f.StringVar(&db, "db", "none", "Database: none, postgres, mysql, sqlite")
	f.StringVar(&access, "access", "driver", "Database access: driver, sqlc, gorm")
	f.BoolVar(&redis, "redis", false, "Include Redis")
	f.BoolVar(&greyship, "greyship", true, "Include Greyship release support")
	f.StringVar(&dir, "dir", "", "Output directory")
	f.BoolVarP(&yes, "yes", "y", false, "Accept defaults without prompts")
	f.BoolVar(&noInstall, "no-install", false, "Skip go mod tidy")
	f.BoolVar(&noGit, "no-git", false, "Skip git init")
	f.BoolVar(&dryRun, "dry-run", false, "Print generated file paths without writing")
	f.BoolVar(&jsonOutput, "json", false, "Print machine-readable output and use defaults")
	return cmd
}

func defaultModule(name string) string {
	user := "me"
	command := exec.Command("git", "config", "user.name")
	if output, err := command.Output(); err == nil {
		candidate := strings.ToLower(strings.TrimSpace(string(output)))
		candidate = strings.ReplaceAll(candidate, " ", "")
		candidate = strings.ReplaceAll(candidate, "/", "")
		if candidate != "" {
			user = candidate
		}
	}
	return "github.com/" + user + "/" + name
}

func prompt(o *plan.Options, cmd *cobra.Command, flags *pflag.FlagSet) error {
	if o.Name == "" {
		if err := huh.NewInput().Title("Project name").Value(&o.Name).Run(); err != nil {
			return err
		}
	}
	if o.Module == "" && !flags.Changed("module") {
		o.Module = defaultModule(o.Name)
	}
	if !flags.Changed("module") {
		if err := huh.NewInput().Title("Module path").Value(&o.Module).Run(); err != nil {
			return err
		}
	}
	if !flags.Changed("router") {
		if err := huh.NewSelect[string]().Title("Router").Options(huh.NewOption("Standard library (net/http)", "stdlib"), huh.NewOption("chi", "chi"), huh.NewOption("Gin", "gin"), huh.NewOption("Echo", "echo"), huh.NewOption("Fiber", "fiber")).Value(&o.Router).Run(); err != nil {
			return err
		}
	}
	if !flags.Changed("db") {
		if err := huh.NewSelect[string]().Title("Database").Options(huh.NewOption("None", "none"), huh.NewOption("PostgreSQL", "postgres"), huh.NewOption("MySQL", "mysql"), huh.NewOption("SQLite", "sqlite")).Value(&o.Database).Run(); err != nil {
			return err
		}
	}
	if o.Database != "none" && !flags.Changed("access") {
		if err := huh.NewSelect[string]().Title("Database access").Options(huh.NewOption("Driver only", "driver"), huh.NewOption("sqlc", "sqlc"), huh.NewOption("GORM", "gorm")).Value(&o.Access).Run(); err != nil {
			return err
		}
	}
	if !flags.Changed("redis") {
		if err := huh.NewConfirm().Title("Include Redis?").Value(&o.Redis).Run(); err != nil {
			return err
		}
	}
	if !flags.Changed("greyship") {
		if err := huh.NewConfirm().Title("Greyship-ready?").Value(&o.Greyship).Run(); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\nProject: %s\nModule: %s\nRouter: %s\nDatabase: %s (%s)\nRedis: %t\nGreyship: %t\n", o.Name, o.Module, o.Router, o.Database, o.Access, o.Redis, o.Greyship); err != nil {
		return err
	}
	confirmed := true
	if err := huh.NewConfirm().Title("Create this project?").Value(&confirmed).Run(); err != nil {
		return err
	}
	if !confirmed {
		return errors.New("project creation cancelled")
	}
	return nil
}
