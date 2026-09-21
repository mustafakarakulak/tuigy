# Architecture

tuigy is a Bubble Tea program in front of the `git` command. There is no daemon, no
index of its own, and no state that outlives the process except the configuration file.

## Packages

```
cmd/tuigy        the command: flags, configuration, starting the program
internal/git     every call to git, and the types its output is parsed into
internal/ui      the interface: model, update, view
internal/keys    every key binding, in one place
internal/config  the optional YAML file
internal/editor  which editor to open a file with
internal/ai      commit messages from a coding agent
```

The dependency direction is one-way: `ui` uses `git`, `keys`, `config`, `editor` and
`ai`; none of them know about `ui`. `git` knows nothing about any of the others, which
is what makes it testable against real repositories without a terminal.

## The git layer

Everything goes through `internal/git.Runner`, which runs the binary and returns
parsed structs — never text meant for display. Two properties are worth knowing:

- **Reads never take the index lock.** They set `GIT_OPTIONAL_LOCKS=0`, so the
  once-a-second status poll cannot block a coding agent working in the same
  repository. See [0003](decisions/0003-poll-rather-than-watch.md).
- **Writes are serialised and retried.** One mutex across the process, plus a bounded
  retry when `.git/index.lock` is held by someone else. An agent committing at the
  same moment is expected, not exceptional.

Output that is parsed uses porcelain formats with `-z` or `%x00` separators, so a path
or a commit subject cannot break the parse. Where git translates its output — the
tracking summary in `for-each-ref` — the command runs under a C locale.

## The interface

One `ui.Model` holds everything. Four tabs share it rather than each owning a
sub-model: they all describe the same repository, and the header, footer and status
are common to all of them.

```
Init        load status, load branches, start the tick
Update      one switch: window size, tick, the result of a git call, a key press
View        header · body (per tab, or a dialog) · footer
```

Every call to git happens in a `tea.Cmd` and arrives back as a message. Nothing blocks
the update loop, which is why a slow push leaves the interface responsive.

Long-running work reports itself: `runOp` sequences a "started" message before the
work, so the footer can say what is happening rather than appearing frozen.

### Messages

Results are matched to what asked for them. A diff carries the row it belongs to, a
commit detail carries its hash, a history page carries its ref and filter. A response
that no longer matches the cursor is dropped rather than rendered, so a slow request
cannot overwrite the pane with something stale.

### Layout

Every pane and dialog is trimmed to its own bounds before being drawn, because
lipgloss pads a box up to a size but does not trim what overflows it. One long line
would otherwise widen the whole layout and push the footer off screen. See
[0010](decisions/0010-clip-every-pane.md).

## Testing

Tests drive the real `git` binary against repositories built in `t.TempDir()`. There
are no mocks of git: the parsing is the risky part, and a mock would only assert that
the code agrees with itself.

Interface tests build a model, send it key messages, and assert on the model and on
the rendered output. `TestMain` forces a colour profile — without a terminal lipgloss
renders everything as plain text, which made several assertions vacuous until it was
noticed.

CI holds total coverage at 90%. The Go version that measures it is pinned; see
[0013](decisions/0013-pin-the-coverage-toolchain.md).
