# 7. Ask git which editor to open, but not for its fallback

Status: accepted

## Context

`e` opens the selected file. Which editor that should be is already a solved question:
git consults `GIT_EDITOR`, then `core.editor`, then `VISUAL`, then `EDITOR`. Reading
those variables directly would miss `core.editor`, which is the only place a great many
people have set one.

`git var GIT_EDITOR` reports the answer. But when nothing is configured it reports
`vi`, and that answer is indistinguishable from someone having chosen `vi`.

Dropping a person into vi who never asked for it is the one outcome worth avoiding
here. They may not know how to leave.

## Decision

Ask git. Separately, determine whether the answer was a choice, by checking whether
`core.editor` or any of the three variables is actually set.

If it was a choice, honour it — including `vi`.

If it was not, look for a friendlier terminal editor that is installed: `micro`, then
`nano`. Fall back to `vi` only when there is nothing else.

tuigy's own `editor:` setting overrides all of it.

## Consequences

Someone who configured an editor gets it. Someone who never thought about it gets
something they can operate without prior knowledge, and the footer names it — `e open
in nano` — so there is no surprise at the moment of pressing the key.

Graphical editors are never chosen automatically. Opening a window from a terminal
tool is presumptuous, and over SSH it would open on the wrong machine. They are one
config line away.

`git var GIT_EDITOR` fails outright on a machine with no editor and a dumb terminal,
which is what a CI container looks like. That is an answer, not an error, and the
function reports it as one.
