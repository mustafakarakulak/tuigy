# 18. Key bindings are changed where they are read

Status: accepted

## Context

The settings screen offered one setting. Everything else it could only point at:
*"Individual colours, key bindings, the editor and the commit message agent live in
this file."* For a keyboard-first tool, the one thing hardest to leave in a file is
which key does what — it is the first thing anyone wants to move, and the answer being
"quit, find a YAML file, learn the action names, restart" is the answer a graphical
program gives.

[8](0008-configuration-is-optional-but-honoured.md) settled how the file is treated.
What it did not settle is whether tuigy writes to it. `SetTheme` already did, for one
line. This is the same question for a block.

## Decision

Settings gets a second page, listing every action with the key it is on, filtered with
`/` because 63 of them is more than anyone scrolls. `enter` waits for a keystroke and
gives it to the action under the cursor; `r` puts the default back.

**One keystroke replaces the binding.** That is what the file already does — `commit:
c` replaces, it does not add — so the screen means the same thing the file means. An
action that wants two keys still needs the file, which the page says.

**Each change is written as it is made,** not gathered up and saved on the way out.
There is nothing to confirm: the binding is either the one you want or it is not, and
you find out by using it. A dialog that saved on exit would also lose everything when
you pressed escape, which here is the key for "I am done looking".

**The file is edited a line at a time.** One line is rewritten or one line is added,
for the same reason `SetTheme` rewrites one line: the file was written by hand, and the
comments beside the bindings you did not touch have no business disappearing because
you moved a different one.

**Escape cannot be captured.** It is the only way out of a mode that swallows every
other keystroke. The file can still rebind it.

**ctrl+c is refused.** It quits before the key map is consulted, so accepting it would
produce a binding that silently never fires. Capture mode does swallow it, though:
someone pressing keys to find out what they are should not have the program exit under
them, and that mode is one keystroke long.

**A key on two actions is reported, not refused.** `enter` checks out a branch, pops a
stash and confirms a dialog, because no two of those are ever on screen at once.
Refusing that would be wrong. Saying nothing would be worse, because the case that is a
mistake looks identical until you are told.

## Consequences

tuigy writes to the user's configuration file in two places now. Both edits are
minimal and both refuse to write a file that would no longer parse.

A comment on the same line as a binding you change is replaced along with it. Comments
above it, below it, and on every other binding survive. This is the one thing the
line-at-a-time edit gives up, and it is bounded.

The running key map is updated before the write, so the footer and the help screen
describe the new binding straight away. They are generated from the map, so nothing
else had to change for that to be true.

The keys page is the second list in tuigy with its own filter that is not the tab's
filter. A third would be a sign that filtering belongs somewhere more general.
