# 3. Poll for changes rather than watch the filesystem

Status: accepted

## Context

The premise of the product is that a coding agent is writing files in another pane and
the view should keep up without a keypress. That can be done by watching the
filesystem with fsnotify, or by asking git what changed on a timer.

Watching sounds cheaper. In practice it means watching the working tree recursively,
which means honouring `.gitignore` to avoid `node_modules`, and watching `.git` for
ref changes, and debouncing both. That is a lot of machinery to answer a question git
answers directly.

## Decision

Run `git status --porcelain=v2` once a second and compare a fingerprint of the result.
Nothing is rebuilt when the fingerprint is unchanged.

## Consequences

Measured cost on a repository with 6000 files and 300 commits: 20–30 ms per status.
At one per second that is a few percent of one core, and it drops to nothing when the
repository is idle because the fingerprint does not change.

Reads set `GIT_OPTIONAL_LOCKS=0`, so polling never takes the index lock and cannot
block the agent it exists to watch.

On a very large monorepo, or a working tree on a network filesystem, this would need
revisiting. The interval is a constant in one place.

One thing polling cannot see: a file rewritten with the same status letters. That
matters for review marks, which is why those are checked separately against the file's
modification time and size. See [0012](0012-review-marks-expire.md).
