// Package config reads tuigy's optional configuration file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is everything a user may set. Every field is optional; an absent file
// is the same as an empty one.
type Config struct {
	// Theme names a built-in theme.
	Theme string `yaml:"theme"`
	// Editor opens a file when "e" is pressed. It overrides git's own setting,
	// and a graphical editor needs whatever flag makes it wait, such as
	// "code --wait".
	Editor string `yaml:"editor"`
	// Colors overrides individual colour roles on top of the theme.
	Colors map[string]string `yaml:"colors"`
	// Keys rebinds actions by name.
	Keys map[string]Keys `yaml:"keys"`
	// AI configures the command that writes commit messages.
	AI AI `yaml:"ai"`
}

type AI struct {
	// Command receives the staged diff on standard input and prints a commit
	// message. The TUIGY_AI_COMMIT environment variable overrides it.
	Command string `yaml:"command"`
}

// Keys is one or more keystrokes. It accepts both a single value and a list,
// because "commit: c" reads better than "commit: [c]" for the common case.
type Keys []string

func (k *Keys) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*k = Keys{s}
		return nil

	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*k = Keys(list)
		return nil

	default:
		return errors.New("expected a key or a list of keys")
	}
}

// Bindings presents the key overrides in the shape the keymap wants.
func (c *Config) Bindings() map[string][]string {
	if len(c.Keys) == 0 {
		return nil
	}
	out := make(map[string][]string, len(c.Keys))
	for action, strokes := range c.Keys {
		out[action] = strokes
	}
	return out
}

// Path is where the configuration file is looked for, honouring XDG_CONFIG_HOME.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "tuigy", "config.yml"), nil
}

// ShortPath writes a path the way a person would say it, with the home
// directory as "~". Configuration paths are long and are shown in places where
// the width is already tight.
func ShortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}

// Load reads the configuration file. A missing file is not an error: tuigy is
// meant to work with no setup at all.
//
// A file that exists but cannot be understood *is* an error. Ignoring it would
// silently disregard what the user asked for, which is worse than saying so.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return &Config{}, nil // no home directory: fall back to defaults
	}
	return loadFile(path)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// config.yaml is accepted too, since both spellings are common.
		if alt := yamlAlternative(path); alt != "" {
			if data, err = os.ReadFile(alt); err == nil {
				path = alt
			}
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	cfg, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func yamlAlternative(path string) string {
	if filepath.Ext(path) == ".yml" {
		return path[:len(path)-len(".yml")] + ".yaml"
	}
	return ""
}
