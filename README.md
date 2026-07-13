# canopy

An interactive test runner for Go that wraps `go test` with test selection, parallel
execution, and a stored history you can browse.

## Sessions and runs

A **run** is one execution of `go test` against a set of packages (its events, results, and
coverage). A **session** is a named group of runs.

In the interactive TUI a session is one launch: you select tests, run them, select more, run
again, all under one session. On the CLI, `canopy test` joins a session *by name* so repeated
runs accumulate into one group you can open and browse together.

### `--session`

`canopy test` takes `--session`, which resolves to a session name. The default is `@branch`.

- `--session <name>`   a literal name, e.g. `--session hotfix-1234`
- `--session @branch`  follow the current git branch (default)
- `--session @module`  follow the go module path
- `--session @worktree` follow the git worktree root

Anything without a leading `@` is used verbatim. If an `@`-resolver can't produce a value
(not a git repo, detached HEAD, etc.) it falls back to `default`.

```
# two runs on the same branch land in one session
canopy test ./foo --store
canopy test ./bar --store
canopy open              # browse both together (opens the current branch's session)

# group an ad-hoc debugging burst under a name of your choosing
canopy test ./... --store --session flaky-hunt
canopy open flaky-hunt
```

### Browsing history

- `canopy list runs`          list stored runs (run IDs on stdout, metadata on stderr)
- `canopy session list [NAME]` list sessions and their run counts
- `canopy show [RUN-ID]`       replay a run's formatted output (defaults to the last run)
- `canopy open [NAME]`         open a session in the interactive UI (defaults to `@branch`)

History is stored in a per-repo `.canopy` SQLite DB, enabled with `--store` (override the
location with `--store-dir`). Retention is controlled by `--max-runs` / `--max-age`.
