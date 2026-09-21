package git

import (
	"context"
	"slices"
	"strings"
)

// Files lists every path git knows about in the working tree: tracked files
// plus untracked ones that .gitignore does not exclude.
//
// This is the repository as git sees it rather than as the filesystem does,
// which is the useful answer for a tree: build output, node_modules and
// vendored dependencies are exactly the noise it would otherwise drown in.
func (r *Repo) Files(ctx context.Context) ([]string, error) {
	out, err := r.run.Read(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	// --cached and --others are two lists concatenated, so a path that is both
	// tracked and reported again can arrive twice.
	seen := make(map[string]struct{})
	paths := make([]string, 0, strings.Count(out, "\x00"))
	for _, p := range strings.Split(out, "\x00") {
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}

	slices.Sort(paths)
	return paths, nil
}
