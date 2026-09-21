# 1. Drive git by shelling out to the binary

Status: accepted

## Context

A Go program can talk to a repository through a library such as go-git, or by running
the `git` command and parsing what it prints.

The library route is tempting: typed results, no parsing, no subprocess. But it is a
reimplementation of git, and it is not complete. Cherry-pick and the merge strategies
are partial. More importantly it does not see the user's setup: credential helpers,
SSH agent configuration, hooks, commit signing, `include` directives in the config,
conditional includes per directory, URL rewriting. All of that is git's, not the
repository format's.

## Decision

Every git operation runs the `git` binary.

Reads run with `GIT_OPTIONAL_LOCKS=0` and writes hold a process-wide mutex with a
retry on `index.lock` contention, because tuigy is designed to run beside a coding
agent working in the same repository.

Parsing uses porcelain formats with NUL separators, and a C locale where git
translates its own output.

## Consequences

Whatever the user has configured works, with no code on our side. A push uses their
credential helper; a commit uses their signing key; their aliases and hooks apply.

The cost is process spawn overhead, which measured at 20–30 ms for a status on a
6000-file repository — small enough that a one-second poll costs a few percent of one
core.

It also means git's own output is an interface we depend on. Porcelain formats are
stable by contract; anything else is not, which is why `for-each-ref` output is read
under a C locale and why status parsing uses `--porcelain=v2 -z` rather than the
human-readable form.

The minimum git version becomes part of the product's contract: 2.23, for `git
switch` and `git restore`. CI runs the suite against 2.30 as well as the current
release.
