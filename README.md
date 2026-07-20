# canopy

An interactive test runner for Go that wraps `go test` with test selection, parallel
execution, and a stored history you can browse.

## Install

```
curl -sSfL https://raw.githubusercontent.com/wagoodman/canopy/main/install.sh | sh
```

The script picks a sensible spot on its own: an existing writable dir on your `PATH` such as `~/.local/bin`, otherwise `/usr/local/bin` (elevating with `sudo` only if needed).

Override the destination with `-b DIR`, verify the release signature with `-v` (requires [cosign](https://docs.sigstore.dev/system_config/installation/)), or pass a release tag to pin a version:

```
curl -sSfL https://raw.githubusercontent.com/wagoodman/canopy/main/install.sh | sh -s -- -v -b /usr/local/bin v0.1.0
```

Or with Go:

```
go install github.com/wagoodman/canopy/cmd/canopy@latest
```

## Quick start

Point canopy at packages the way you would `go test`. It discovers the tests, drops you into
an interactive picker to choose what to run, runs the selection, and shows the results:

```
canopy ./...
```

Add `--store` to persist the run, then reopen the session later to page through failures,
output, and coverage without re-running:

```
canopy test ./... --store
canopy open
```

There are two interactive surfaces:

- the **selector** (a bare `canopy ./...`) is where you pick which tests to run
- the **studio** (`canopy open`) browses a session's runs: navigate by package or function,
  filter to just failures, read a test's output, and re-run a selection in place

Everything below is optional depth: how runs are grouped, how to diagnose failures, and how to
make a flaky failure reproducible.

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

## Reproducibility (`--shuffle` and repros)

`canopy test --shuffle` randomizes test and benchmark order (Go's `-shuffle`). Instead of
letting the toolchain pick a seed, canopy generates one, passes it, and records it with the
run, so the recorded value is authoritative: replay it and the execution order is identical.

With `--store`, every run captures an execution fingerprint alongside its results: the shuffle
seed, `-race`, `-count`, build `-tags`, the Go version / GOOS / GOARCH, and an allowlisted slice
of env (`GOFLAGS`, `CGO_ENABLED`, plus anything you name). `triage --show-repros` (and `verify`'s
JSON) then emit a `go test` command per failure that recreates those conditions:

```
canopy test ./foo --shuffle --store
canopy triage --show-repros
#   …/TestThing
#   go test ./foo -shuffle=on -test.shuffle=1784084485868271000 -run '^TestThing$'
```

Run that command yourself, or hand it to a teammate or an agent, and it fails the same way, in
the same order, under the same flags.

- a run with `--race` shows `-race` in the repro; a run without `--shuffle` emits a repro with no
  seed; a run without `--store` persists no fingerprint and falls back to the plain
  `go test ... -run` form.
- `--repro-env KEY,KEY2` folds extra env keys into the fingerprint on top of the built-in
  allowlist, so `MY_FLAG=xyz canopy test ./foo --repro-env MY_FLAG --store` yields a repro
  prefixed `MY_FLAG=xyz go test ...`. Only named (and allowlisted) keys are captured, never your
  whole environment.
- a repro recorded under a different toolchain carries a trailing `# recorded under go1.x
  linux/amd64` note, since a shuffle seed only reproduces under the same toolchain.

Use case: pin down a flaky or order-dependent failure. Shuffle until it fails, then the recorded
seed turns that one-in-N failure into a command that fails on demand.

## More commands

Beyond selecting and running tests, canopy reads its stored history to answer questions about a
codebase over time. All of these need `--store`d runs to draw from.

- `canopy affected [GO-PKG-SPECIFIER...]`   report which tests are affected by a change, using
  the static import graph (what to re-run after editing a symbol)
- `canopy coverage [RUN-ID]`   show coverage for the last run (or a specific one)
- `canopy trend flaky`         detect flaky tests across historical sessions
- `canopy trend failures`      failure-rate trends over time
- `canopy trend duration`      test duration trends
- `canopy trend count`         test-suite size over time

Run any command with `--help` for its flags.
