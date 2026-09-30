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

- `canopy list runs`              list stored runs (`-o id` for bare run IDs, `-o json` for scripting)
- `canopy list sessions [NAME]`   list sessions and the runs grouped under each
- `canopy show [RUN-ID]`          replay a run's formatted output (defaults to the last run)
- `canopy open [NAME]`            open a session in the interactive UI (defaults to `@branch`)

History is stored in a per-repo `.canopy` SQLite DB, enabled with `--store` (override the
location with `--store-dir`). Retention comes from the `store.max-runs` / `store.max-age`
config keys, applied by `canopy db prune` (or override them per-invocation with
`--keep-last` / `--older-than`).

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

## Sharding in CI

`canopy test` can split a package set across a CI matrix and then verify that the pieces add up. You can adopt it one level at a time, and each level works on its own:

1. **No matrix.** `canopy test ./...`, as usual.
2. **Matrix.** Add `--shard i/n`. No extra jobs, no state.
3. **Verifying join.** Each shard uploads a small receipt, and one `canopy shard join` job checks the whole run.
4. **Metrics.** Cached timings balance the split better, and canopy suggests a shard count.

What runs is always decided from `go list` on the current checkout, never from config or cached state. State can only make the split better balanced, not less correct.

### Level 2: matrix

```yaml
jobs:
  test:
    runs-on: ubuntu-24.04-arm
    strategy:
      fail-fast: false
      matrix:
        shard: [1, 2, 3, 4]
    name: test (shard ${{ matrix.shard }}/${{ strategy.job-total }})
    steps:
      - uses: actions/checkout@...
      - run: canopy test ./... --shard ${{ matrix.shard }}/${{ strategy.job-total }}
```

`i/n` is 1-based. The split is deterministic and balanced by test counts. Gates like `test.covermin` are recorded in the shard's receipt and left to the join, since one shard's coverage only covers its own packages. Use `fail-fast: false`, otherwise a failing shard cancels the others and the join reports them missing.

### Level 3: verifying join

Upload each shard's receipt, then add a job that downloads them all and runs `canopy shard join`. Make this job the single required check.

```yaml
      # added to the test job
      - uses: actions/upload-artifact@...
        if: always()
        with: { name: "canopy-shard-${{ matrix.shard }}", path: .canopy/shard/out/ }

  tests:
    needs: test
    if: always()
    runs-on: ubuntu-latest
    steps:
      - run: curl -sSfL https://raw.githubusercontent.com/wagoodman/canopy/main/install.sh | sh -s -- v0.1.0
      - uses: actions/download-artifact@...
        with: { pattern: "canopy-shard-*", path: .canopy/shard/out/, merge-multiple: true }
      - run: canopy shard join
```

The join needs no checkout and no Go toolchain. It is configured like `canopy test`: same `.canopy.yaml` `test:` section, same `CANOPY_TEST_*` env vars, same flags. A checked-in `test.covermin: 80` applies to a bare `canopy shard join`, and `--covermin 90` overrides it. With no config at all, it enforces whatever gates the shards recorded.

Pin the same canopy version on the shard jobs and the join job (the `v0.1.0` tag above). The version is part of the input digest, so shards on different versions fail the join with a `[plan]` mismatch, and a join on a different version than its shards warns.

#### What the join verifies

- exactly n receipts, indices 1 to n
- every shard ran with the same inputs: package selection, build and test flags, gates, Go toolchain and env, commit, canopy version, and the split itself (shard count and weights). Only the shard index differs
- the shards' packages split the full package list with no gaps and no overlap
- every package a shard planned actually reported a result
- no shard failed, then the merged coverage against `covermin`

If any of that fails the join fails, even when every test passed. A package that never ran anywhere is exactly the failure this exists to catch.

#### Exit codes

| Code | Meaning | Examples |
|---|---|---|
| 0 | everything passed | |
| 1 | tests failed (same as `canopy test`) | a shard reported a failure |
| 2 | couldn't evaluate | no receipts, an unreadable receipt, bad flags |
| 3 | verification failed, results can't be trusted | missing shard, different inputs, a package that never ran or ran twice |
| 4 | a gate failed | coverage below `covermin`, shards disagreeing on a gate |

When several apply, the most fundamental one wins (2, 3, 4, then 1), and the output still lists all of them. Every failure line names its shard (`shard 3/4`) and, for different inputs, the exact lines that differed.

#### Output formats

`canopy shard join -o` works like `canopy test -o` and can be repeated:

- `text` is the default
- `json` is the full report as a versioned JSON contract
- `github-summary` appends markdown to the step summary (bare, it uses `$GITHUB_STEP_SUMMARY`; `=path` appends to a file)

```
canopy shard join -o text -o json=join.json -o github-summary
```

With no `-o`, you get `text`, plus `github-summary` when `$GITHUB_STEP_SUMMARY` is set. Every output is written before the join exits, even on failure, so `join.json` is there for whatever runs next.

### Level 4: metrics

Shards and the join restore a cached `metrics.json`, and the join on main saves it back.

```yaml
      # added before `canopy test` in the test job, and before `canopy shard join` in the join job
      - uses: actions/cache/restore@...
        with:
          path: .canopy/shard/metrics.json
          key: canopy-shard-${{ runner.os }}-${{ runner.arch }}-${{ github.run_id }}
          restore-keys: canopy-shard-${{ runner.os }}-${{ runner.arch }}-

      # added after `canopy shard join` in the join job
      - if: always() && github.ref == 'refs/heads/main'
        uses: actions/cache/save@...
        with:
          path: .canopy/shard/metrics.json
          key: canopy-shard-${{ runner.os }}-${{ runner.arch }}-${{ github.run_id }}
```

Packages with a measured time use it, and the rest get their test count converted to time at the rate the measured ones run. Missing, corrupt or foreign metrics make every shard fall back to test counts the same way, so losing the cache costs balance and nothing else. The join also prints a shard count suggestion with estimated wall and runner time for each option. It is only advice, so you edit the matrix list by hand.

Caching notes:

- only the join on main saves; shards and PR joins only read
- cache entries can't be overwritten, so each save gets a fresh key (by `run_id`) and restores take the newest one by prefix
- to reset, bump the key prefix
- aging out is passive. GitHub evicts entries that go unread for 7 days, the file keeps at most 5 samples per package, packages missing from the last 10 saves are dropped, and an environment change starts the file over. No cleanup job and no `actions: write`

If the join reports a `[weights]` mismatch, use **Re-run all jobs**, not "Re-run failed jobs". Re-running one shard after main saved new metrics gives that shard a different split than its siblings. The same message shows up if a cache save lands while a run's shards are starting.

### `--shard auto`

Most CI providers tell a parallel job which copy it is through env vars. `--shard auto` reads them, so the config only says how many copies to run:

| Provider | How you ask for N copies | What canopy reads |
|---|---|---|
| GitLab | `parallel: 4` | `CI_NODE_INDEX` (1-based), `CI_NODE_TOTAL` |
| CircleCI | `parallelism: 4` | `CIRCLE_NODE_INDEX` (0-based), `CIRCLE_NODE_TOTAL` |
| Buildkite | `parallelism: 4` | `BUILDKITE_PARALLEL_JOB` (0-based), `BUILDKITE_PARALLEL_JOB_COUNT` |
| Azure Pipelines | `strategy: { parallel: 4 }` | `SYSTEM_JOBPOSITIONINPHASE`, `SYSTEM_TOTALJOBSINPHASE` |
| GitHub Actions | `matrix` | nothing, use the explicit form above |

```yaml
# GitLab
test:
  parallel: 4
  script:
    - canopy test ./... --shard auto
```

- it's opt-in. Plain `canopy test` never shards, even inside a parallel job
- with no parallelism variables it resolves to `1/1` and runs everything, so one job template works whether or not the job is parallel
- an index without a valid total is an error, not a guess
- the log says where the value came from: `shard 2/4 (from CI_NODE_INDEX/CI_NODE_TOTAL)`
- on GitHub you can keep the same command line everywhere by setting `CANOPY_TEST_SHARD: ${{ matrix.shard }}/${{ strategy.job-total }}` in the job's `env:` and leaving the flag off

Don't use `auto` with GitLab `parallel: matrix:`. It sets `CI_NODE_INDEX`/`CI_NODE_TOTAL` across every combination, so packages get split across your matrix values. Use plain `parallel: N` with one job per group.

### Other matrix dimensions

Go versions, OS, `-race` and build tags each make a separate **shard group**, and each group gets verified on its own. A few rules keep them apart:

1. **Pass an explicit total.** `strategy.job-total` counts every combination, so `go: [1.26, 1.27]` with three shards is 6, not 3. Write `--shard ${{ matrix.shard }}/3`. A wrong total still gets caught by the join (a missing shard or an invalid index).
2. **Namespace the artifacts** by the other dimensions, e.g. `canopy-shard-go${{ matrix.go }}-${{ matrix.shard }}`. Otherwise `merge-multiple` lets one group's `shard-1.json` overwrite another's.
3. **One join per group.** Give the join job the same non-shard dimensions and have it download only its group's artifacts. For a single required check, add a final job that `needs:` the join matrix.
4. **One metrics cache key per group** when groups run differently, e.g. add `race${{ matrix.race }}` to the key. A Go version can share a key.

If two groups share a cache key anyway, the metrics file records a profile of how tests run (`-race`, `-tags`, `-count`) and a different profile ignores the file. Both groups fall back to test counts instead of reading each other's timings.

```yaml
jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        go: ["1.26", "1.27"]
        shard: [1, 2, 3]
    name: test (go ${{ matrix.go }}, shard ${{ matrix.shard }}/3)
    runs-on: ubuntu-24.04-arm
    steps:
      - uses: actions/checkout@...
      - uses: actions/setup-go@...
        with: { go-version: "${{ matrix.go }}" }
      - run: canopy test ./... --shard ${{ matrix.shard }}/3
      - uses: actions/upload-artifact@...
        if: always()
        with: { name: "canopy-shard-go${{ matrix.go }}-${{ matrix.shard }}", path: .canopy/shard/out/ }

  join:
    needs: test
    if: always()
    strategy:
      fail-fast: false
      matrix:
        go: ["1.26", "1.27"]
    name: tests (go ${{ matrix.go }})
    runs-on: ubuntu-latest
    steps:
      - run: curl -sSfL https://raw.githubusercontent.com/wagoodman/canopy/main/install.sh | sh -s -- v0.1.0
      - uses: actions/download-artifact@...
        with: { pattern: "canopy-shard-go${{ matrix.go }}-*", path: .canopy/shard/out/, merge-multiple: true }
      - run: canopy shard join

  tests:                      # optional: one required check for all groups
    needs: join
    if: always()
    runs-on: ubuntu-latest
    steps:
      - run: test "${{ needs.join.result }}" = success
```

If groups do get mixed in one join, it fails with a mismatch in the group that differs: `[go]` for Go version or OS/arch, `[run]` for `-race`, `-tags` or `-run`. An env var your tests read is only part of that check if you list it in `--repro-env`.

### Trying it locally

`canopy shard plan` prints the plan without running anything: packages and estimated load per shard, the input digest, and the shard count suggestion. It takes the same package selection and config as `canopy test`, so it matches what a shard would compute.

```
canopy shard plan ./... --shards 4
canopy shard plan ./... -o json
```

Without `--shards`, only the suggestion is printed. `-o` takes `text` (default) or `json`.

### Combined event log (optional)

The join only needs receipts. If you also want one view of every test result across all shards, each shard can write its raw `go test -json` stream, and `canopy format` replays the concatenation as if it had been one run. Shards run disjoint packages, so the streams just concatenate.

```yaml
  test:
    steps:
      - run: canopy test ./... --shard ${{ matrix.shard }}/${{ strategy.job-total }} -o go -o json=.canopy/shard/out/shard-${{ matrix.shard }}.jsonl
      # the same upload-artifact step picks up the .jsonl files along with the receipts

  tests:
    steps:
      # ... download artifacts, canopy shard join ...
      - if: always()
        run: cat .canopy/shard/out/*.jsonl | canopy format --store --store-dir .canopy/combined --session ci-${{ github.run_id }}
      - if: always()
        uses: actions/upload-artifact@...
        with: { name: canopy-combined, path: .canopy/combined/ }
```

Download `canopy-combined` and browse it locally with `canopy open ci-<run id> --store-dir <dir>`. `canopy triage` and `canopy verify` work on it too. `canopy format` also takes `-o` formats if you want a report file from the combined stream.

Caveats:

- it's a convenience, not a check. The join never looks at event files, and receipts are what guarantee every package ran
- event logs include all test output and get big, so compress them before upload
- elapsed time in the combined view is the replay, not the run. The join's report has the real per-shard times
- `canopy format` exits 1 when the stream has failures, hence `if: always()` on those steps while the join stays the gate

## More commands

Beyond selecting and running tests, canopy reads its stored history to answer questions about a
codebase over time. All of these need `--store`d runs to draw from.

- `canopy affected [GO-PKG-SPECIFIER...]`   report which tests are affected by a change, using
  the static import graph (what to re-run after editing a symbol)
- `canopy coverage [RUN-ID]`   show coverage for the last run (or a specific one)
- `canopy trend flaky`         detect flaky tests across historical sessions
- `canopy trend duration`      test duration trends
- `canopy trend count`         test-suite size over time

Run any command with `--help` for its flags.
