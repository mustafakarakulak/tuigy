# 9. Themes are a palette of roles

Status: accepted

## Context

A theme could be a mapping for every element on screen, or a small set of colours that
every element is derived from.

## Decision

Eight roles: text, muted, border, accent, added, removed, warning, inverse. Every
style in the interface is built from them, and a theme is a choice of those eight.

Built-in themes commit to fixed colours. The default adapts to the terminal's own
background, so it looks deliberate on a light and a dark terminal alike.

Individual roles can be overridden on top of a theme, as hex values.

## Consequences

Writing a theme is eight decisions rather than forty, and a new element on screen
cannot invent a colour that no theme accounts for.

The styles are package-level and rebuilt as a set, so a theme is applied atomically and
no style is ever left over from a previous one. That makes live preview in the settings
screen free: moving the cursor repaints everything.

Terminal palette indexes are deliberately not accepted from a config file. They mean
something different in every terminal, which is exactly what a theme is trying to pin
down.
