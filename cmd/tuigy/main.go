// Command tuigy is a keyboard-first TUI for managing git without leaving the terminal.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/ai"
	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/editor"
	"github.com/mustafakarakulak/tuigy/internal/git"
	"github.com/mustafakarakulak/tuigy/internal/keys"
	"github.com/mustafakarakulak/tuigy/internal/ui"
)

// Set by the linker for release builds; a local build leaves them alone.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "tuigy:", err)
		os.Exit(1)
	}
}

// cli handles the command line and is separate from main so that the flags,
// which are part of the tool's contract, can be tested.
func cli(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("tuigy", flag.ContinueOnError)
	flags.SetOutput(out)

	showVersion := flags.Bool("version", false, "print version information and exit")
	flags.BoolVar(showVersion, "v", false, "print version information and exit")
	listActions := flags.Bool("actions", false, "list the action names a config file can rebind")
	listThemes := flags.Bool("themes", false, "list the built-in themes")
	showConfig := flags.Bool("config", false, "print the path of the configuration file")
	initConfig := flags.Bool("init-config", false, "write a documented configuration file, if there is none")

	if err := flags.Parse(args); err != nil {
		// Asking for help is not a failure.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	switch {
	case *showVersion:
		fmt.Fprintln(out, buildInfo())
		return nil
	case *listActions:
		fmt.Fprintln(out, strings.Join(keys.ActionNames(), "\n"))
		return nil
	case *listThemes:
		fmt.Fprintln(out, strings.Join(ui.ThemeNames(), "\n"))
		return nil

	case *showConfig:
		path, err := config.Path()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, path)
		return nil

	case *initConfig:
		path, created, err := config.WriteStarter()
		if err != nil {
			return err
		}
		if !created {
			fmt.Fprintf(out, "%s already exists, leaving it alone\n", path)
			return nil
		}
		fmt.Fprintf(out, "wrote %s\n", path)
		return nil
	}

	return run()
}

func run() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := git.Open(context.Background(), wd)
	if err != nil {
		return errors.New("must be run inside a git repository")
	}

	model, err := configure(repo)
	if err != nil {
		return err
	}

	// AltScreen leaves the user's scrollback exactly as it was on exit.
	p := tea.NewProgram(model, tea.WithAltScreen())
	final, err := p.Run()

	// A shell left running in the terminal pane is closed before we go, so the
	// user gets their terminal back with nothing still holding on to it.
	if m, ok := final.(ui.Model); ok {
		m.Shutdown()
	}
	return err
}

// configure applies the user's configuration file.
//
// A broken configuration stops startup rather than being ignored: carrying on
// with defaults would silently disregard what the user asked for, and the
// message here says both where the file is and what is wrong with it.
func configure(repo *git.Repo) (tea.Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	path, _ := config.Path()

	if err := ui.ApplyTheme(cfg.Theme, cfg.Colors); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	keyMap := keys.Default()
	if err := keyMap.Apply(cfg.Bindings()); err != nil {
		return nil, fmt.Errorf("%s: %w (tuigy --actions lists them)", path, err)
	}

	fromGit, chosen := repo.EditorCommand(context.Background())

	return ui.New(repo,
		ui.WithKeys(keyMap),
		ui.WithAI(ai.Detect(cfg.AI.Command)),
		ui.WithEditor(editor.Resolve(cfg.Editor, fromGit, chosen)),
		ui.WithTheme(cfg.Theme),
	), nil
}

// buildInfo describes the running binary.
//
// Release builds carry these through ldflags. A binary from `go install` has
// no ldflags but the toolchain records the same facts, so they are read back
// rather than reporting nothing useful in a bug report.
func buildInfo() string {
	v, c, d := version, commit, date

	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if mv := info.Main.Version; mv != "" && mv != "(devel)" {
				v = mv
			}
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					c = setting.Value[:min(len(setting.Value), 7)]
				case "vcs.time":
					d = setting.Value
				}
			}
		}
	}

	return fmt.Sprintf("tuigy %s (%s, built %s)", v, c, d)
}
