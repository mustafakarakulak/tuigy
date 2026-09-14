// Package editor decides which editor tuigy hands a file to.
package editor

import (
	"os/exec"
	"strings"
)

// friendly are the editors tried when nothing at all has been configured.
//
// They are ordered by how usable each is to someone who has never opened it:
// nano prints its own key bindings along the bottom, micro uses the shortcuts
// every other program uses. Both are terminal editors, so they keep a terminal
// tool in the terminal and work the same way over SSH.
var friendly = []string{"micro", "nano"}

// noOp are the commands git uses to mean "do not open an editor at all".
// Opening one of these would make the key appear to do nothing.
var noOp = map[string]bool{"": true, "true": true, ":": true, "/usr/bin/true": true}

// Resolve returns the command used to open a file.
//
// The order is deliberate:
//
//  1. tuigy's own setting, which is the most specific thing the user said.
//  2. git's editor, but only when it was actually chosen. git falls back to vi,
//     and dropping someone into vi who never asked for it is the one outcome
//     worth avoiding here.
//  3. A friendly editor that is actually installed.
//  4. Whatever git said after all, so the behaviour still matches git's.
func Resolve(configured, fromGit string, chosen bool) string {
	if c := strings.TrimSpace(configured); c != "" {
		return c
	}

	fromGit = strings.TrimSpace(fromGit)
	if chosen && !noOp[fromGit] {
		return fromGit
	}

	for _, candidate := range friendly {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	if noOp[fromGit] {
		return "vi"
	}
	return fromGit
}

// Name is the editor's name without its path or arguments, for showing the user.
func Name(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}

	name := fields[0]
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	return name
}
