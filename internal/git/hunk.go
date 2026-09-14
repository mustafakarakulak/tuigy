package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Hunk is one contiguous block of changes within a file's diff.
type Hunk struct {
	// Header is the @@ line. git needs it back exactly as it was given.
	Header string
	// Body is the lines the hunk covers, context included.
	Body []string
}

// Lines is the hunk as git wrote it.
func (h Hunk) Lines() []string { return append([]string{h.Header}, h.Body...) }

// SplitHunks separates a file diff into the header git apply needs and the
// hunks it can be narrowed down to.
//
// Everything before the first @@ is the header: the diff line, the index line
// and the two file names. A patch is only valid with all of it.
func SplitHunks(diff string) (header []string, hunks []Hunk) {
	for line := range strings.SplitSeq(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			hunks = append(hunks, Hunk{Header: line})
		case len(hunks) == 0:
			header = append(header, line)
		default:
			last := &hunks[len(hunks)-1]
			last.Body = append(last.Body, line)
		}
	}
	return header, hunks
}

// HunkPatch builds a patch containing one hunk of a file diff.
//
// Applying it moves only that part of the file between the working tree and the
// index. The line numbers in the hunk header are left untouched: they describe
// where the change sits in the original file, which is exactly what git apply
// needs to find it.
func HunkPatch(diff string, index int) (string, error) {
	header, hunks := SplitHunks(diff)

	switch {
	case len(hunks) == 0:
		return "", errors.New("this change has no hunks to stage")
	case index < 0 || index >= len(hunks):
		return "", fmt.Errorf("there is no hunk %d", index+1)
	case len(header) == 0:
		return "", errors.New("this diff has no header, so it cannot be applied")
	}

	lines := append(append([]string{}, header...), hunks[index].Lines()...)
	return strings.Join(lines, "\n") + "\n", nil
}

// ApplyToIndex applies a patch to the index, leaving the working tree alone.
//
// Reversing it is how a hunk is taken back out of the index, which is the same
// operation read the other way round.
func (r *Repo) ApplyToIndex(ctx context.Context, patch string, reverse bool) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("nothing to apply")
	}

	args := []string{"apply", "--cached"}
	if reverse {
		args = append(args, "--reverse")
	}
	args = append(args, "-")

	_, err := r.run.WriteStdin(ctx, patch, args...)
	return err
}

// StageHunk moves one hunk of a file's unstaged changes into the index.
func (r *Repo) StageHunk(ctx context.Context, diff string, index int) error {
	patch, err := HunkPatch(diff, index)
	if err != nil {
		return err
	}
	return r.ApplyToIndex(ctx, patch, false)
}

// UnstageHunk takes one hunk back out of the index.
func (r *Repo) UnstageHunk(ctx context.Context, diff string, index int) error {
	patch, err := HunkPatch(diff, index)
	if err != nil {
		return err
	}
	return r.ApplyToIndex(ctx, patch, true)
}
