# 10. Clip every pane and dialog to its own bounds

Status: accepted

## Context

Lipgloss pads a box up to a width and a height. It does not trim content that exceeds
them: the box grows instead.

In a layout built from panes joined side by side, one over-long line therefore widens
the entire layout, and one extra row pushes the footer off the screen. The failure is
not local to the thing that overflowed.

This was found by a test asserting the view is exactly as tall and wide as the
terminal, which failed at 80 columns because a footer of shortcuts was 97 wide.

## Decision

Everything drawn into a pane or a dialog goes through `fitPane`, which trims to the
box before lipgloss sees it. Trimming is ANSI-aware: cutting by runes would slice an
escape sequence in half and leave the terminal in a broken colour state.

The footer, which cannot simply be cut, drops shortcuts from the end instead — with
help and quit reserved, so the way out of a screen is never what truncation removes.

## Consequences

Every view is exactly the size of the terminal, which is asserted at five widths from
120 columns down to 40, for every tab and every dialog.

The diff pane is the one deliberate exception. Its lines are left whole so the viewport
can scroll horizontally; the viewport does the cutting, and it is ANSI-aware too.
