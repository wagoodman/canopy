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

## `triage` vs `verify`

Both look at test failures, but they differ by how many runs each reads.

- `triage` diagnoses **one** run. For each failure it decides flaky / pre-existing /
  new-regression, collapses the failures into distinct symptoms, and (when a diff is
  present) points at the changed symbol that explains them. It has no baseline, so it
  never reports what got *fixed*, it describes what is wrong now and why. Descriptive.
- `verify` diffs a run against a **baseline** run and answers one yes/no: did my change
  fix its target and break nothing new? That second run is what lets it report a `fixed`
  bucket and emit an `ok` boolean plus a matching exit code. A gate.

Mental model: `triage` answers "what's wrong and why", `verify` answers "did I fix it
without breaking anything". They pair up, `triage` a failing run to understand it, edit,
then `verify` to confirm you're done.

When to reach for which:

| Situation | Command |
|---|---|
| A run failed and you want to understand it (real vs flaky, distinct problems, likely cause) | `triage` |
| You just edited code and want to know "am I done?" (target fixed, zero new regressions) | `verify` |
| CI gate: did this PR make things worse (diff against main's baseline, pass/fail) | `verify` |
| An agent triaging failures it didn't cause (separate yours from flaky/pre-existing, find the cause) | `triage` |
