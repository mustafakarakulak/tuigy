package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// themeLine matches the theme setting at the start of a line, which is the only
// place a top-level key can be.
var themeLine = regexp.MustCompile(`(?m)^theme:.*$`)

// SetTheme records a theme choice in the configuration file.
//
// The file is edited rather than regenerated: it was written by hand, so its
// comments, ordering and anything tuigy does not understand all survive. Only
// the one line changes, and a line is appended when there is none.
func SetTheme(name string) (path string, err error) {
	path, err = Path()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return path, err
	}

	line := []byte("theme: " + name)
	if themeLine.Match(data) {
		data = themeLine.ReplaceAll(data, line)
	} else {
		if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
			data = append(data, '\n')
		}
		data = append(data, append(line, '\n')...)
	}

	// Never write a file tuigy would then refuse to start with.
	if _, err := parse(data); err != nil {
		return path, fmt.Errorf("the edit would make %s unreadable: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, data, 0o644)
}
