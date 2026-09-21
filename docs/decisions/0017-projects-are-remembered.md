# 17. Every repository is remembered, and switching happens in place

Status: accepted

## Context

The audience [15](0015-a-workspace-around-the-git-tool.md) describes runs agents in
several checkouts at once. Moving between them meant quitting tuigy, changing
directory and starting it again, which throws away everything the session was holding:
the review marks, the filter, the history page that was loaded, the shell that was
mid-command.

Two questions follow. How does a repository get into a list of repositories, and what
happens to the interface when you pick one.

## Decision

**Opening a repository records it.** There is no "save this project" step. A switcher
whose list has to be curated is one more thing to maintain, and the list it would end
up holding is the same list. The entries go in `~/.config/tuigy/projects.yml`,
honouring `XDG_CONFIG_HOME`, ordered by when each was last opened and capped at 50.

**Separate from `config.yml`.** That file is written by hand and tuigy only ever edits
the one line it owns; this one is rewritten by the program and has nothing in it worth
hand-editing.

**A broken projects file is not fatal.** This is a deliberate departure from
[8](0008-configuration-is-optional-but-honoured.md), where an unreadable configuration
stops startup because ignoring it would silently disregard what the user asked for.
Nobody asked for this file. It is tuigy's own bookkeeping, and refusing to start over
it — or over a `$HOME` that cannot be written to — would be absurd. It is reported,
and the next write repairs it.

**Switching replaces the repository in the running process.** Everything derived from
the old one is dropped rather than reloaded over it: a cursor, a filter, a review mark
or a loaded diff carried across would be describing a repository that is no longer on
screen. What the user chose — the theme, the key map, the editor — stays.

## Consequences

tuigy writes a file the user did not ask for. It holds paths, names and timestamps,
nothing about the contents of any repository. `D` in the switcher forgets one, and
deleting the file loses nothing but the ordering.

The repository already open is left out of the list, because switching to where you
are is not something anyone means to do.

A remembered path can have been moved or deleted since. That is reported in the footer
rather than leaving `enter` looking like it does nothing.

Two instances of tuigy in two repositories write to the same file. The list is read
again each time the switcher opens rather than trusted from startup, so the second one
to open sees what the first recorded. A write that lands between the two is lost from
the ordering, which costs nobody anything.

The terminal band closes on a switch; see [16](0016-the-terminal-is-a-band.md).
