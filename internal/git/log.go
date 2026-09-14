package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Commit is one entry in the history.
type Commit struct {
	Hash    string
	Short   string
	Subject string
	Author  string
	Date    time.Time
	// Refs is git's own decoration string, such as "HEAD -> main, origin/main".
	Refs    string
	Parents []string
}

// IsMerge reports a commit with more than one parent.
func (c Commit) IsMerge() bool { return len(c.Parents) > 1 }

// logFormat is NUL-separated so that a subject containing anything at all
// cannot break the parse. The body comes last and keeps whatever it holds.
const logFormat = "%H%x00%h%x00%s%x00%an%x00%ct%x00%D%x00%P"

// Log lists commits reachable from ref, newest first.
//
// It is paginated rather than read in full: a long-lived repository has tens of
// thousands of commits and the view shows a screenful.
//
// A non-empty grep narrows the result to commits whose message contains it,
// matched by git across the whole history rather than only the loaded page.
func (r *Repo) Log(ctx context.Context, ref, grep string, skip, limit int) ([]Commit, error) {
	if ref == "" {
		ref = "HEAD"
	}
	if limit <= 0 {
		return nil, nil
	}

	// A repository with no commits has no HEAD to log, which is not an error.
	if skip == 0 {
		if _, err := r.run.Read(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
			if ExitCode(err) == 1 {
				return nil, nil
			}
			return nil, err
		}
	}

	args := []string{"log", "--format=" + logFormat, "-n" + strconv.Itoa(limit)}
	if skip > 0 {
		args = append(args, "--skip="+strconv.Itoa(skip))
	}
	if grep != "" {
		args = append(args, "--regexp-ignore-case", "--fixed-strings", "--grep="+grep)
	}
	args = append(args, ref, "--")

	out, err := r.run.Read(ctx, args...)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if c, ok := parseCommit(line); ok {
			commits = append(commits, c)
		}
	}
	return commits, nil
}

func parseCommit(line string) (Commit, bool) {
	f := strings.SplitN(line, "\x00", 7)
	if len(f) < 7 {
		return Commit{}, false
	}

	c := Commit{
		Hash:    f[0],
		Short:   f[1],
		Subject: f[2],
		Author:  f[3],
		Refs:    f[5],
	}
	if secs, err := strconv.ParseInt(f[4], 10, 64); err == nil {
		c.Date = time.Unix(secs, 0)
	}
	if parents := strings.Fields(f[6]); len(parents) > 0 {
		c.Parents = parents
	}
	return c, true
}

// CommitFile is one file touched by a commit.
type CommitFile struct {
	Status   StatusCode
	Path     string
	OrigPath string
}

// CommitDetail is everything the history view shows about one commit.
type CommitDetail struct {
	Commit Commit
	Body   string
	Files  []CommitFile
	Diff   string
}

// CommitDetail reads the full message, file list and diff of one commit.
func (r *Repo) CommitDetail(ctx context.Context, hash string) (*CommitDetail, error) {
	if hash == "" {
		return nil, errors.New("no commit given")
	}

	// %B comes last so the body keeps every byte, separators included.
	out, err := r.run.Read(ctx, "show", "--no-patch", "--format="+logFormat+"%x00%B", hash)
	if err != nil {
		return nil, err
	}

	fields := strings.SplitN(strings.TrimRight(out, "\n"), "\x00", 8)
	if len(fields) < 8 {
		return nil, fmt.Errorf("unexpected output for commit %s", hash)
	}

	commit, ok := parseCommit(strings.Join(fields[:7], "\x00"))
	if !ok {
		return nil, fmt.Errorf("could not parse commit %s", hash)
	}

	detail := &CommitDetail{Commit: commit, Body: strings.TrimSpace(fields[7])}

	if detail.Files, err = r.commitFiles(ctx, hash); err != nil {
		return nil, err
	}
	if detail.Diff, err = r.readDiff(ctx, "show", "--format=", hash); err != nil {
		return nil, err
	}
	return detail, nil
}

// renameStatus matches a rename or copy status, which is a letter followed by a
// similarity score. A merge commit's status is letters only — one per parent —
// so the digits are what tells the two apart.
var renameStatus = regexp.MustCompile(`^[RC][0-9]+$`)

func (r *Repo) commitFiles(ctx context.Context, hash string) ([]CommitFile, error) {
	out, err := r.run.Read(ctx, "show", "--name-status", "--format=", "-z", hash)
	if err != nil {
		return nil, err
	}

	fields := strings.Split(out, "\x00")
	var files []CommitFile

	for i := 0; i < len(fields); i++ {
		status := fields[i]
		if status == "" {
			continue
		}

		file := CommitFile{Status: StatusCode(status[0])}

		if renameStatus.MatchString(status) {
			// A rename carries both the old and the new path.
			if i+2 >= len(fields) {
				break
			}
			file.OrigPath, file.Path = fields[i+1], fields[i+2]
			i += 2
		} else {
			if i+1 >= len(fields) {
				break
			}
			file.Path = fields[i+1]
			i++
		}

		files = append(files, file)
	}

	return files, nil
}

// CherryPick applies commits to the branch that is currently checked out.
//
// They are applied in the order given. The history lists commits newest first,
// so a caller passing a selection straight from the view would replay them
// backwards; they must be reversed into chronological order first.
func (r *Repo) CherryPick(ctx context.Context, hashes ...string) error {
	if len(hashes) == 0 {
		return errors.New("no commits selected")
	}
	_, err := r.run.Write(ctx, append([]string{"cherry-pick"}, hashes...)...)
	return err
}

// CherryPickInto applies commits to target, checking it out first when it is
// not the branch already in use. As with a merge, git offers no way to commit
// onto a branch you are not on.
func (r *Repo) CherryPickInto(ctx context.Context, target, current string, hashes ...string) error {
	if target != current {
		if err := r.Checkout(ctx, target); err != nil {
			return err
		}
	}
	return r.CherryPick(ctx, hashes...)
}
