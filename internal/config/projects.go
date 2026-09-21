package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// maxProjects bounds the remembered list. It is a switcher, not an archive:
// past a certain length the list stops being something you can scan and the
// filter is doing all the work anyway.
const maxProjects = 50

// Project is a repository tuigy has been opened in.
type Project struct {
	// Path is the absolute root of the working tree, and identifies the entry.
	Path string `yaml:"path"`
	// Name is what the switcher shows, which is the directory's own name.
	Name string `yaml:"name"`
	// Opened orders the list: most recent first, so the switcher opens on the
	// repository you are most likely to want next.
	Opened time.Time `yaml:"opened"`
}

// projectFile is the on-disk shape. The list is wrapped in a document rather
// than written as a bare sequence so that the file has room to grow a setting
// later without breaking anything that already reads it.
type projectFile struct {
	Projects []Project `yaml:"projects"`
}

// ProjectsPath is where the remembered repositories are kept.
//
// It is a separate file from config.yml deliberately: config.yml is written by
// hand and tuigy only ever edits the one line it owns, whereas this file is
// rewritten by the program and has nothing in it worth hand-editing.
func ProjectsPath() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "projects.yml"), nil
}

// LoadProjects reads the remembered repositories, most recently opened first.
//
// Unlike the configuration file, a broken projects file is not fatal: it is
// tuigy's own bookkeeping, not something the user asked for, and refusing to
// start over it would be absurd. It is reported so the caller can say so.
func LoadProjects() ([]Project, error) {
	path, err := ProjectsPath()
	if err != nil {
		return nil, err
	}
	return loadProjectsFile(path)
}

func loadProjectsFile(path string) ([]Project, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var file projectFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	kept := file.Projects[:0]
	for _, p := range file.Projects {
		if p.Path != "" {
			kept = append(kept, p)
		}
	}
	sortProjects(kept)
	return kept, nil
}

// RememberProject records that a repository was opened, and returns the list as
// it now stands.
//
// Every repository tuigy is started in is remembered, without being asked. The
// alternative is a "save this project" step, which is one more thing to know
// about for a list whose whole purpose is to have been filled in already.
func RememberProject(root, name string) ([]Project, error) {
	return updateProjects(func(list []Project) []Project {
		entry := Project{Path: root, Name: name, Opened: time.Now()}

		out := make([]Project, 0, len(list)+1)
		out = append(out, entry)
		for _, p := range list {
			if p.Path != root {
				out = append(out, p)
			}
		}
		return out
	})
}

// ForgetProject drops a repository from the list. Nothing on disk is touched:
// this removes an entry from a switcher, not a repository from the machine.
func ForgetProject(root string) ([]Project, error) {
	return updateProjects(func(list []Project) []Project {
		return slices.DeleteFunc(list, func(p Project) bool { return p.Path == root })
	})
}

func updateProjects(apply func([]Project) []Project) ([]Project, error) {
	path, err := ProjectsPath()
	if err != nil {
		return nil, err
	}

	// A file that cannot be read is treated as empty rather than as a reason to
	// refuse: losing the history is a smaller harm than a switcher that stops
	// working and cannot repair itself.
	list, _ := loadProjectsFile(path)

	list = apply(list)
	sortProjects(list)
	if len(list) > maxProjects {
		list = list[:maxProjects]
	}

	return list, saveProjectsFile(path, list)
}

func saveProjectsFile(path string, list []Project) error {
	data, err := yaml.Marshal(projectFile{Projects: list})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// sortProjects orders by recency, with the path breaking ties so that a file
// written twice in the same instant still has a stable order.
func sortProjects(list []Project) {
	slices.SortStableFunc(list, func(a, b Project) int {
		if !a.Opened.Equal(b.Opened) {
			if a.Opened.After(b.Opened) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
}
