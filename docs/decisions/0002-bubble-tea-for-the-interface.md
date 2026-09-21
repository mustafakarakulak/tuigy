# 2. Bubble Tea for the interface

Status: accepted

## Context

A terminal interface in Go can be built on tcell, on Bubble Tea, or by hand.

## Decision

Bubble Tea, with Lipgloss for styling and Bubbles for the viewport, text input and
spinner.

## Consequences

The model-update-view shape suits a program whose state is "what git last told us"
and whose input is keystrokes and command results. Every git call becomes a `tea.Cmd`
and every result a message, which is what keeps the interface responsive during a slow
push without any concurrency of our own.

Lipgloss has one behaviour worth knowing about, which cost an afternoon: it pads a box
up to a width but does not trim content that exceeds it. See
[0010](0010-clip-every-pane.md).

The viewport handles horizontal scrolling and is ANSI-aware about it, which is only
useful if lines are not cut before they reach it.
