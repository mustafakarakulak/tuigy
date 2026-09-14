# tuigy

*English · [Türkçe](README.tr.md)*

A fast, keyboard-first Git TUI for managing branches, commits, diffs, merges, and cherry-picks without leaving your terminal.

tuigy is built for developers working alongside AI coding agents. Run your agent in one
terminal pane and tuigy in another: the file list and diffs refresh on their own as the
agent writes, so you can review, stage and commit without touching an IDE.

It is deliberately **not** a full Git client. The goal is to make the handful of
operations you actually perform every day as fast and as obvious as possible.

## Status

Everything on the original plan is in. Working today:

- repository header: branch, upstream, ahead/behind, dirty state, in-progress merge or cherry-pick
- changes view split into conflicts / staged / unstaged / untracked, grouped by directory
  where that saves repeating a prefix
- diff viewer for working tree, index and untracked files: scrolls in both directions,
  picks out the words that actually changed within an edited line, and jumps hunk to hunk
- stage, unstage, stage all, unstage all, discard (with confirmation), and stage or
  unstage a single hunk from inside the diff
- commit and amend
- branch list with per-branch upstream, ahead/behind and last commit
- switch branch, check out a remote branch, create a branch from a chosen source, delete a branch
- fetch, fetch all, pull and push
- merge with an explicit source → target, stating up front whether it merges in place,
  fast-forwards, or has to check the target branch out
- conflict handling: the conflicted files are listed, opened in your editor, staged as
  resolved, then the merge or cherry-pick is continued or aborted
- commit history for any branch, with each commit's message, files and diff; `/` searches
  it through git, so a match older than the loaded page is still found
- cherry-pick: select one or more commits, choose the branch to apply them to, and they
  are replayed in the order they were written
- stashes: save the working tree away with a message, browse what each stash holds,
  then pop, apply or drop it
- AI commit messages: `ctrl+g` in the commit view hands the staged diff to a coding
  agent you already run, and puts its draft in the editable field
- live refresh: changes made by another process show up without a keypress
- review marks: tick off files as you read an agent's changes, and the tick disappears
  again if the file is rewritten underneath you
- filter any list with `/`, and copy a path, branch, commit hash or stash ref with `Y`
- themes and key bindings you can change

## Install

Prebuilt binaries for Linux and macOS are attached to every
[release](https://github.com/mustafakarakulak/tuigy/releases).

tuigy drives the `git` binary, so it needs git 2.23 or newer (for `git switch`
and `git restore`). CI runs the test suite against git 2.30 and current git, on
Linux and macOS.

Windows is not supported: opening a file in your editor and generating a commit
message both run through a POSIX shell.

With Go 1.24.2 or newer:

```sh
go install github.com/mustafakarakulak/tuigy/cmd/tuigy@latest
```

Or from a clone:

```sh
go build -o tuigy ./cmd/tuigy
```

`tuigy --version` reports which build you are running, which is worth including
in a bug report.

## Usage

Run `tuigy` anywhere inside a Git repository.

| Key | Action |
| --- | --- |
| `1` `2` `3` `4` | changes / branches / history / stashes tab |
| `↑` `↓` / `k` `j` | move |
| `g` / `G` | jump to first / last |
| `ctrl+d` / `ctrl+u` | scroll the diff or detail pane |
| `←` `→` / `h` `l` | scroll a diff sideways |
| `]` / `[` | jump to the next / previous hunk |
| `space` (in a diff) | stage or unstage the hunk under the cursor |
| `esc` | close a dialog, leave a pane, clear a filter |
| `/` | filter the list you are on |
| `Y` | copy what the cursor is on (path, branch, commit hash, stash) |
| `,` | settings: pick a theme, see where everything else lives |
| `?` | help (scrollable) |
| `r` | refresh |
| `q` | quit |

Changes tab:

| Key | Action |
| --- | --- |
| `tab` | switch between the file list and the diff |
| `enter` | focus the diff |
| `space` | stage or unstage the selected file |
| `s` / `u` | stage / unstage |
| `a` / `A` | stage all / unstage all |
| `d` | discard changes (asks first) |
| `v` | mark the file as reviewed |
| `e` | open the selected file in your editor |
| `S` | stash your changes |
| `c` / `C` | commit / amend the last commit |
| `ctrl+g` | commit, with the message written by a coding agent |
| `ctrl+s` | confirm the commit message |

Branches tab:

| Key | Action |
| --- | --- |
| `enter` | switch to the selected branch |
| `n` | create a branch from the selected one |
| `m` | merge the selected branch into a branch you pick |
| `l` | show that branch's history |
| `D` | delete the selected local branch (asks first) |

History tab:

| Key | Action |
| --- | --- |
| `enter` | focus the commit detail |
| `space` | select the commit for a cherry-pick, and step down |
| `y` | cherry-pick the selection onto a branch you pick |

Stashes tab:

| Key | Action |
| --- | --- |
| `enter` | pop the stash |
| `a` | apply it, keeping the stash |
| `D` | drop it (asks first) |

Anywhere:

| Key | Action |
| --- | --- |
| `f` / `F` | fetch / fetch all remotes |
| `p` / `P` | pull / push |
| `m` | while a merge or cherry-pick is unfinished: continue or abort it |

Staging a conflicted file marks it as resolved, the same as `git add`.

## Configuration

tuigy works with no configuration at all.

Press `,` to change the theme: the whole view is repainted as you move through
them, and `enter` writes the choice to your configuration file. That screen also
says where the file is and what else lives in it.

For everything else:

```sh
tuigy --init-config   # write a documented configuration file
tuigy --config        # print where it is
tuigy --themes        # list the built-in themes
tuigy --actions       # list every action a key can be bound to
```

The file is `~/.config/tuigy/config.yml`, or `$XDG_CONFIG_HOME/tuigy/config.yml`:

```yaml
# One of: default, dracula, gruvbox, nord. Press "," inside tuigy to try them.
theme: dracula

# Any of these can be overridden on top of the theme, as a hex colour.
colors:
  accent: "#bd93f9"   # current branch, focused pane, selected row
  added: "#50fa7b"    # added lines, new files, a branch that is ahead
  removed: "#ff5555"  # removed lines, deleted files, errors
  warning: "#ffb86c"  # dirty tree, a branch that is behind, unfinished merge
  text: "#f8f8f2"
  muted: "#6272a4"    # labels, hints, metadata
  border: "#44475a"
  inverse: "#282a36"  # text drawn on top of accent, removed or warning

# What "e" opens. Overrides git's own setting. A graphical editor needs
# whatever flag makes it wait, or tuigy carries on while it is still open.
editor: "code --wait"

# Rebind any action. `tuigy --actions` lists every name.
# A single key or a list of them; "space" means the space bar.
keys:
  commit: [c, ctrl+k]
  quit: Q
  stash-push: w

ai:
  command: my-agent --headless "write a conventional commit message"
```

A configuration file that cannot be understood stops tuigy with a message saying which
file and what is wrong with it, rather than starting up and quietly ignoring it. Saving a
theme from the settings screen edits only that one line, so comments and anything else you
wrote survive.

**Fonts are not tuigy's to set.** A terminal program writes characters; which typeface
draws them belongs to your terminal emulator. What tuigy does choose is the handful of
symbols it uses — arrows, bullets, box drawing — which need a font with reasonable
Unicode coverage.

## Staging part of a file

With the diff focused, `]` and `[` move between hunks and `space` moves the one under the
cursor across on its own. The file then sits in both the staged and the unstaged list,
which is exactly what has happened to it.

This is worth having next to a coding agent, which rarely produces a file whose every
change belongs in the same commit. Untracked files are the exception: there is nothing to
apply a patch against yet, so they are staged whole.

## Reviewing what an agent wrote

Press `v` to tick a file off as read. The tick shows in the list, the header counts how
far through you are, and the commit view says when something staged has not been looked at.

The tick is dropped again if the file changes. That is the point of it: an agent that
rewrites a file you already read has invalidated the reading, and a status poll cannot see
that on its own — the file is still just "modified".

It is a note to yourself, not a gate. Nothing refuses to commit.

## Which editor `e` opens

In order: tuigy's own `editor:` setting, then whatever git would use
(`GIT_EDITOR`, `core.editor`, `VISUAL`, `EDITOR`), and only then a default.

That last step is the interesting one. git's own fallback is `vi`, which is a
surprising place to land if you never chose it, so tuigy looks for a friendlier
terminal editor first — `micro`, then `nano` — and uses `vi` only when there is
nothing else. Anything you actually configured is always honoured, including
`vi` itself.

The key's own hint names the editor, so the footer reads `e open in nano` rather
than leaving you to find out by pressing it.

```yaml
editor: "code --wait"   # or: cursor --wait, zed --wait, nvim, hx, …
```

A graphical editor needs the flag that makes it wait for the file to be closed.
Without it tuigy carries on immediately; the change still shows up, since the
view refreshes on its own, but nothing pauses for you.

## Commit messages from a coding agent

Stage what you want to commit, then press `ctrl+g`. It opens the commit view, hands the
staged diff to a coding agent, and puts the message it writes into the editable field.
`ctrl+g` again asks for a different one; `ctrl+s` commits; `esc` throws it away.

Nothing is committed until you confirm, so the message is a draft you read and edit — the
same thing you would have typed, written for you.

tuigy does not talk to a model provider itself: no API key, no billing, no SDK to keep up
with. It runs a command you already have. `claude` is used automatically when it is on
your `PATH`, and the key hint names whichever agent will answer, so the footer reads
`ctrl+g write with claude`. Anything else goes in the config file, or in the environment,
which takes precedence:

```sh
export TUIGY_AI_COMMIT='my-agent --headless "write a commit message"'
```

The command is given the diff on standard input and is expected to print the message.
If no agent is found, the key is not offered at all.

## Design notes

**tuigy shells out to the `git` binary** rather than using a Git library. Your existing
config, credential helpers, SSH agent, hooks and signing setup work with no extra
plumbing. Reads run with `GIT_OPTIONAL_LOCKS=0` so polling never blocks a coding agent
working in the same repository, and writes are serialised with a retry on `index.lock`
contention.

Git itself never gets to open a pager, an editor or a terminal prompt while tuigy is
driving it. Anything slow — a fetch, a pull, a push over a poor connection — says what it
is doing while it runs, in the same place the result then appears. A push to an HTTPS remote
without a credential helper fails fast with a visible error instead of hanging the UI.

**Nothing surprising happens on your behalf.** `pull` is fast-forward only, so a divergent
branch is reported rather than quietly resolved with a merge commit you did not ask for.
When local changes block a branch switch, tuigy offers to stash them and says so; it never
stashes on its own. A merge into a branch you are not on says beforehand whether it can be
written straight to the ref or has to check that branch out and leave you there.

## Development

Nothing here needs `make` — `go build ./...` and `go test ./...` work on their own. The
Makefile is a convenience, and `make` on its own lists what it offers:

```sh
make check       # formatting, vet and the suite: what to run before pushing
make test        # the suite
make race        # the suite under the race detector
make cover       # coverage across packages, and the 90% floor CI enforces
make cover-html  # and where the gaps are
make linux       # run the suite on Linux, where CI runs it (needs Docker)
make snapshot    # build the release artefacts without publishing
make build       # ./tuigy
```

The tests drive the real `git` binary against throwaway repositories, so they exercise
the same code paths the tool does rather than a mock of them. `make linux` is worth
running before anything to do with the shell or the filesystem: it has caught behaviour
that differs between macOS and the container CI uses.

CI runs the same suite on Linux and macOS, against the oldest supported git as well as
the current one, and calls `make cover` for the coverage floor so the number is defined
in one place. That job pins its Go version: 1.27 changed how coverage blocks are
recorded, and the same tests over the same code report 88.8% on 1.24 and 91.4% on 1.27,
so a floor is only meaningful next to the toolchain that measured it.

## License

MIT
