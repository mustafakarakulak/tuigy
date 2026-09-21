package config

import (
	"errors"
	"os"
	"path/filepath"
)

// starter is written by --init-config. It is entirely commented out, so writing
// it changes nothing about how tuigy behaves; its job is to say what can be
// changed, which is otherwise only discoverable by reading the documentation.
const starter = `# tuigy configuration
#
# Everything here is optional. Uncomment what you want to change.

# Colours. One of: default, dracula, gruvbox, nord.
# "tuigy --themes" lists them, and "," inside tuigy tries them out live.
# theme: dracula

# Override individual colours on top of the theme, as hex values.
# colors:
#   accent: "#bd93f9"    # current branch, focused pane, selected row
#   added: "#50fa7b"     # added lines, new files, a branch that is ahead
#   removed: "#ff5555"   # removed lines, deleted files, errors
#   warning: "#ffb86c"   # dirty tree, a branch that is behind, unfinished merge
#   text: "#f8f8f2"
#   muted: "#6272a4"     # labels, hints, metadata
#   border: "#44475a"
#   inverse: "#282a36"   # text drawn on top of accent, removed or warning

# What "e" opens. Overrides git's own setting. A graphical editor needs the flag
# that makes it wait, or tuigy carries on while the file is still open.
# editor: "code --wait"

# Rebind any action. "tuigy --actions" lists every name, and "," inside tuigy
# changes them one at a time by pressing the key you want.
# A single key or a list of them; "space" means the space bar.
# keys:
#   commit: [c, ctrl+k]
#   quit: Q

# Write commit messages with a coding agent. The command is given the staged
# diff on standard input and is expected to print the message.
# The TUIGY_AI_COMMIT environment variable overrides this.
# ai:
#   command: "my-agent --headless 'write a conventional commit message'"
`

// WriteStarter creates a documented configuration file and reports where.
//
// An existing file is never touched: the point is to help someone get started,
// not to overwrite what they have already written.
func WriteStarter() (path string, created bool, err error) {
	path, err = Path()
	if err != nil {
		return "", false, err
	}

	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return path, false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		return path, false, err
	}
	return path, true, nil
}
