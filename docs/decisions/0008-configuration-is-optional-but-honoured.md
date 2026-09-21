# 8. Configuration is optional, and a broken file stops startup

Status: accepted

## Context

tuigy should work with no setup. It should also let people change the theme, the key
bindings, the editor and the agent command.

That leaves a question about a file that exists but cannot be understood: a typo in a
theme name, an action that does not exist, YAML that does not parse.

## Decision

YAML at `~/.config/tuigy/config.yml`, honouring `XDG_CONFIG_HOME`. An absent file is
the same as an empty one.

A file that cannot be honoured stops startup, with the path and what is wrong with it.

## Consequences

Starting anyway and ignoring the file would mean quietly disregarding what the user
asked for, which is worse than a message they can act on. The message names the file
and, where it applies, what the valid values are.

Two things follow from the file being hand-written. Saving a theme from the settings
screen edits only that one line, so comments and anything tuigy does not understand
survive. And `tuigy --init-config` writes a file that is entirely comments: it changes
no behaviour, and exists so that the options are discoverable without reading
documentation.
