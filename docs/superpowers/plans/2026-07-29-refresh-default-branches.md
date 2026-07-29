# Automatic Default-Branch Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GitM automatically detect and cache repository default-branch changes before every command whose behavior depends on the default branch.

**Architecture:** Query the authoritative `origin` symbolic `HEAD` with `git ls-remote --symref`, then reconcile the result into each selected repository's in-memory model and SQLite cache. Default-sensitive commands invoke one central reconciler; an unavailable remote produces an aggregated warning and leaves the cached value usable.

**Tech Stack:** Go, Cobra, SQLite, the real `git` executable, and the existing runner/concurrency patterns.

## Global Constraints

- Use `origin` as the authoritative remote.
- Query repositories concurrently and persist changed cache values sequentially.
- Remote lookup failures use the cached branch and emit one concise aggregated warning.
- Database persistence failures are returned and stop the command.
- Dry-run commands use the live result in memory but do not persist it.
- Explicit branch arguments bypass default-branch refresh when the default branch is not otherwise needed.
- Add no external dependencies and no database migration.
- Git-operation tests must use real temporary repositories, never mocks.
- Every new function must have tests; verification is `go test ./... -v -race -timeout 180s`, `make lint`, and `make test`.

---

### Task 1: Authoritative Remote Default Detection

**Files:**
- Modify: `internal/git/git.go`
- Test: `internal/git/git_more_test.go`

**Interfaces:**
- Produces: `git.RemoteDefaultBranch(path string) (string, error)`.
- Updates: `git.DefaultBranch(path string) (string, error)` to try the remote helper before its existing local fallbacks.

- [ ] Write a real-repository regression test where a bare remote starts on `main`, retains both branches, changes symbolic `HEAD` to `master`, and the local clone still has stale `origin/HEAD` pointing to `main`.
- [ ] Run the focused test and confirm it fails because `RemoteDefaultBranch` is missing.
- [ ] Implement parsing of `git ls-remote --symref origin HEAD`, accepting only `ref: refs/heads/<branch>\tHEAD` and returning contextual errors for missing or malformed symbolic HEAD output.
- [ ] Verify the remote helper returns `master` despite the stale local tracking symbolic ref.
- [ ] Add fallback coverage proving `DefaultBranch` still uses local `origin/HEAD`, local `main`/`master`, then current `HEAD` when remote detection is unavailable.
- [ ] Run `go test ./internal/git -v -race` and commit the task.

### Task 2: Reconcile Cached Defaults Across Commands

**Files:**
- Create: `internal/cli/default_branch.go`
- Test: `internal/cli/default_branch_test.go`
- Modify: `internal/cli/checkout.go`, `internal/cli/sync.go`, `internal/cli/branch_run.go`, `internal/cli/commit.go`, `internal/cli/branches.go`, `internal/cli/update.go`, `internal/cli/doctor.go`, `internal/cli/repo.go`
- Test: relevant existing `internal/cli/*_test.go` files

**Interfaces:**
- Consumes: `git.RemoteDefaultBranch(path string) (string, error)` and `database.UpdateDefaultBranch(alias, branch)`.
- Produces: one internal reconciliation helper that updates repository objects, optionally persists changes, and reports aggregated lookup warnings.

- [ ] Write failing real-repository tests proving reconciliation changes stale `main` to remote `master` in memory and SQLite, falls back with a warning when origin is unavailable, and does not persist in dry-run mode.
- [ ] Implement concurrent remote lookup, deterministic aggregated warnings, sequential persistence, and contextual persistence errors.
- [ ] Wire refresh into default checkout; implicit sync; branch creation without `--from`; branch delete protection; commit protection for dirty repos; branch dashboard calculations; update's missing-upstream fallback; doctor; and repository listing.
- [ ] Ensure explicit checkout/sync/create bases skip unnecessary refresh and dry-run previews use live in-memory values without database writes.
- [ ] Add focused integration tests for each call site, including protection of newly detected `master` and switching/branching from `master` while stale `main` still exists.
- [ ] Run `go test ./internal/cli -v -race` and commit the task.

### Task 3: Documentation and End-to-End Verification

**Files:**
- Modify: `README.md`
- Modify: CLI help text where default-branch behavior is described
- Test: `internal/e2e/*_test.go` when needed for command-level coverage

**Interfaces:**
- No public command syntax changes. `gitm checkout main` and `gitm checkout master` remain aliases for default-branch mode.

- [ ] Document live default detection, cache updates, offline warning/fallback, and dry-run non-persistence in the existing command reference.
- [ ] Add an end-to-end regression that registers a repository on `main`, changes remote symbolic `HEAD` to `master` while both exist, and verifies a default operation follows `master` without re-registering.
- [ ] Run formatting and focused tests, then run `go test ./... -v -race -timeout 180s`, `make lint`, and `make test`.
- [ ] Commit documentation and end-to-end coverage.
