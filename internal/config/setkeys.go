package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// keysLine matches the keys block's own line, which like any top-level key can
// only be at the start of a line.
var keysLine = regexp.MustCompile(`^keys:\s*(#.*)?$`)

// SetKeyBinding records a key binding in the configuration file.
//
// Like SetTheme, the file is edited rather than regenerated, and the edit is as
// small as it can be: one line is rewritten or one line is added. Every comment,
// every blank line and every setting tuigy does not understand survives, including
// the comments beside the bindings the user did not touch.
func SetKeyBinding(action string, strokes []string) (path string, err error) {
	if len(strokes) == 0 {
		return ResetKeyBinding(action)
	}
	return editConfig(func(lines []string) []string {
		return setKeyLine(lines, action, yamlStrokes(strokes))
	})
}

// ResetKeyBinding drops an override, which puts the action back to the key it
// ships with the next time tuigy starts.
func ResetKeyBinding(action string) (path string, err error) {
	return editConfig(func(lines []string) []string {
		return removeKeyLine(lines, action)
	})
}

// editConfig applies a line edit to the configuration file and writes it back.
func editConfig(edit func([]string) []string) (path string, err error) {
	path, err = Path()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return path, err
	}

	lines := splitLines(string(data))
	updated := strings.Join(edit(lines), "\n")

	// Never write a file tuigy would then refuse to start with.
	if _, err := parse([]byte(updated)); err != nil {
		return path, fmt.Errorf("the edit would make %s unreadable: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(updated), 0o644)
}

// splitLines cuts a file into lines, keeping a trailing empty one so that
// joining puts the final newline back.
func splitLines(data string) []string {
	if data == "" {
		return []string{""}
	}
	return strings.Split(data, "\n")
}

// keysBlock locates the keys mapping: the line it opens on, and the line after
// the last entry in it. It reports ok false when the file has no keys block.
func keysBlock(lines []string) (start, end int, indent string, ok bool) {
	start = -1
	for i, line := range lines {
		if keysLine.MatchString(line) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, "", false
	}

	// The block runs to the last indented, non-blank line under it. Blank lines
	// inside are part of it; blank lines after it are not.
	end, indent = start+1, ""
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !isIndented(line) {
			break
		}
		if indent == "" {
			indent = leadingSpace(line)
		}
		end = i + 1
	}

	if indent == "" {
		indent = "  "
	}
	return start, end, indent, true
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func leadingSpace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// entryLine matches one action inside the keys block, commented out or not, so
// that setting a binding the starter file mentions replaces that line rather
// than leaving a contradictory comment above the real setting.
func entryLine(indent, action string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(indent) + `#?\s*` + regexp.QuoteMeta(action) + `:`)
}

// setKeyLine writes one action's keys, adding the block if there is none.
func setKeyLine(lines []string, action, value string) []string {
	start, end, indent, ok := keysBlock(lines)
	if !ok {
		return appendBlock(lines, action, value)
	}

	entry := indent + action + ": " + value
	pattern := entryLine(indent, action)
	for i := start + 1; i < end; i++ {
		if pattern.MatchString(lines[i]) {
			lines[i] = entry
			return lines
		}
	}

	// Not there yet: it goes at the end of the block, where the eye looks for
	// what was added last.
	return insertAt(lines, end, entry)
}

// removeKeyLine takes one action out, and takes the block with it when that
// leaves nothing behind. A keys block with no entries under it is not valid
// YAML for our shape, so it cannot simply be left empty.
func removeKeyLine(lines []string, action string) []string {
	start, end, indent, ok := keysBlock(lines)
	if !ok {
		return lines
	}

	pattern := entryLine(indent, action)
	kept := make([]string, 0, len(lines))
	remaining := 0
	for i, line := range lines {
		if i > start && i < end {
			if pattern.MatchString(line) {
				continue
			}
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				remaining++
			}
		}
		kept = append(kept, line)
	}

	if remaining == 0 {
		// Drop the now-empty block, which is the line it opened on.
		withoutHeader := make([]string, 0, len(kept))
		for i, line := range kept {
			if i == start && keysLine.MatchString(line) {
				continue
			}
			withoutHeader = append(withoutHeader, line)
		}
		return withoutHeader
	}
	return kept
}

// appendBlock adds a keys block to a file that has none.
func appendBlock(lines []string, action, value string) []string {
	out := append([]string{}, lines...)

	// A file that does not end in a newline gets one, and a block that follows
	// content gets a blank line so the file stays readable.
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}

	return append(out, "keys:", "  "+action+": "+value, "")
}

func insertAt(lines []string, at int, line string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, line)
	return append(out, lines[at:]...)
}

// yamlStrokes writes keystrokes the way the file reads them back.
//
// Every value is quoted: a keystroke can be ",", "?" or "-", each of which means
// something else to a YAML parser left to guess.
func yamlStrokes(strokes []string) string {
	quoted := make([]string, 0, len(strokes))
	for _, s := range strokes {
		if s == " " {
			// The help text calls it "space", and a lone space in a file is
			// something nobody can see.
			s = "space"
		}
		quoted = append(quoted, strconv.Quote(s))
	}

	if len(quoted) == 1 {
		return quoted[0]
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
