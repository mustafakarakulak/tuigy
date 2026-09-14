package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// StatusCode is a porcelain v2 single-letter status code.
type StatusCode byte

const (
	StatusUnmodified StatusCode = '.'
	StatusModified   StatusCode = 'M'
	StatusAdded      StatusCode = 'A'
	StatusDeleted    StatusCode = 'D'
	StatusRenamed    StatusCode = 'R'
	StatusCopied     StatusCode = 'C'
	StatusTypeChange StatusCode = 'T'
	StatusUntracked  StatusCode = '?'
)

func (c StatusCode) String() string {
	if c == StatusUnmodified {
		return " "
	}
	return string(byte(c))
}

// FileStatus is one file's state in the index and in the working tree.
//
// Index and Worktree are independent: a file reported as "MM" belongs in both
// the staged and the unstaged list, because it genuinely has changes in each.
type FileStatus struct {
	Path     string
	OrigPath string // previous path for a rename or copy

	Index    StatusCode
	Worktree StatusCode

	Unmerged bool
}

// Status is the full state of the repository at one point in time.
type Status struct {
	Branch   string // empty when HEAD is detached
	Upstream string
	Head     string // short commit hash; empty before the first commit
	Ahead    int
	Behind   int
	Detached bool

	Files []FileStatus
}

// Status reads the current state of the repository.
//
// git's default handling of untracked files is kept: a wholly untracked
// directory is reported as a single entry. --untracked-files=all would produce
// tens of thousands of lines in a repository with an untracked node_modules.
func (r *Repo) Status(ctx context.Context) (*Status, error) {
	out, err := r.run.Read(ctx, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return nil, err
	}
	return parseStatus(out)
}

func parseStatus(out string) (*Status, error) {
	s := &Status{}

	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		line := fields[i]
		if line == "" {
			continue
		}

		switch line[0] {
		case '#':
			parseBranchHeader(s, line)

		case '1':
			f, err := parseEntry(line, 9, 8)
			if err != nil {
				return nil, err
			}
			s.Files = append(s.Files, f)

		case '2':
			f, err := parseEntry(line, 10, 9)
			if err != nil {
				return nil, err
			}
			// Under -z the original path of a rename or copy is the next field.
			if i+1 < len(fields) {
				i++
				f.OrigPath = fields[i]
			}
			s.Files = append(s.Files, f)

		case 'u':
			f, err := parseEntry(line, 11, 10)
			if err != nil {
				return nil, err
			}
			f.Unmerged = true
			s.Files = append(s.Files, f)

		case '?':
			s.Files = append(s.Files, FileStatus{
				Path:     line[2:],
				Index:    StatusUnmodified,
				Worktree: StatusUntracked,
			})

		case '!':
			// ignored files are not shown
		}
	}

	return s, nil
}

// parseEntry splits one space-delimited porcelain v2 entry. All three entry
// types share a shape; only the field count and the index of the path differ.
func parseEntry(line string, nFields, pathIdx int) (FileStatus, error) {
	parts := strings.SplitN(line, " ", nFields)
	if len(parts) < nFields || len(parts[1]) < 2 {
		return FileStatus{}, fmt.Errorf("malformed status entry: %q", line)
	}
	return FileStatus{
		Path:     parts[pathIdx],
		Index:    StatusCode(parts[1][0]),
		Worktree: StatusCode(parts[1][1]),
	}, nil
}

func parseBranchHeader(s *Status, line string) {
	key, val, ok := strings.Cut(strings.TrimPrefix(line, "# "), " ")
	if !ok {
		return
	}

	switch key {
	case "branch.oid":
		// A repository with no commits yet reports "(initial)".
		if val != "(initial)" {
			s.Head = val[:min(len(val), 7)]
		}
	case "branch.head":
		if val == "(detached)" {
			s.Detached = true
		} else {
			s.Branch = val
		}
	case "branch.upstream":
		s.Upstream = val
	case "branch.ab":
		ahead, behind, ok := strings.Cut(val, " ")
		if !ok {
			return
		}
		s.Ahead, _ = strconv.Atoi(strings.TrimPrefix(ahead, "+"))
		s.Behind, _ = strconv.Atoi(strings.TrimPrefix(behind, "-"))
	}
}

// Staged lists files with changes in the index.
func (s *Status) Staged() []FileStatus {
	return s.filter(func(f FileStatus) bool {
		return !f.Unmerged && f.Worktree != StatusUntracked && f.Index != StatusUnmodified
	})
}

// Unstaged lists tracked files with changes in the working tree.
func (s *Status) Unstaged() []FileStatus {
	return s.filter(func(f FileStatus) bool {
		return !f.Unmerged && f.Worktree != StatusUntracked && f.Worktree != StatusUnmodified
	})
}

// Untracked lists files and directories git does not track yet.
func (s *Status) Untracked() []FileStatus {
	return s.filter(func(f FileStatus) bool { return f.Worktree == StatusUntracked })
}

// Conflicted lists files left unmerged by a merge or cherry-pick.
func (s *Status) Conflicted() []FileStatus {
	return s.filter(func(f FileStatus) bool { return f.Unmerged })
}

// IsClean reports whether the working tree has no changes at all.
func (s *Status) IsClean() bool { return len(s.Files) == 0 }

func (s *Status) filter(keep func(FileStatus) bool) []FileStatus {
	var out []FileStatus
	for _, f := range s.Files {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}
