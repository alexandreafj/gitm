# Fast Repository Listing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `gitm repo list` render cached repository metadata without contacting Git remotes.

**Architecture:** Keep SQLite as the listing source and remove default-branch reconciliation only from the active-context `repo list` path. Operational commands continue using the existing reconciliation helper when they require live remote-default information.

**Tech Stack:** Go, Cobra, SQLite, the real `git` binary, shell-backed Git external transport for the regression test.

## Global Constraints

- Do not add dependencies.
- Use real temporary Git repositories and the actual `git` binary; never mock Git operations.
- Every new function must have tests.
- Wrap errors with context and do not introduce global state.
- Update CLI help and `README.md` when command behavior changes.
- Run formatting, lint, and the race-enabled test suite before committing.

---

### Task 1: Prove listing does not contact remotes

**Files:**
- Modify: `internal/cli/repo_test.go`

**Interfaces:**
- Consumes: `repoListCmd() *cobra.Command`, `setupTestDB`, `initRepoWithRemote`, and the real Git external-transport mechanism.
- Produces: `TestRepoListUsesCachedDefaultBranchWithoutContactingRemote`, which fails whenever `repo list` invokes the repository's remote transport.

- [x] **Step 1: Replace the refresh expectation with a cache-only regression test**

Create a real temporary repository and bare origin, then replace its origin URL with an `ext::` transport script that creates a marker and blocks until a release file exists. Run `repoListCmd().RunE` and assert it completes with the cached `main` value before the transport marker appears. Release the transport during cleanup so a failing implementation cannot leak a child process.

- [x] **Step 2: Run the focused test to verify it fails**

Run:

```bash
GOCACHE=/tmp/gitm-repo-list-gocache GOTMPDIR=/tmp go test ./internal/cli -run '^TestRepoListUsesCachedDefaultBranchWithoutContactingRemote$' -count=1 -v
```

Expected: FAIL because the current implementation invokes the blocking remote transport.

### Task 2: Make listing cache-only

**Files:**
- Modify: `internal/cli/repo.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: cached `default_branch` values returned by `ListRepositoriesByContext` and `ListRepositories`.
- Produces: active-context and `--all` listings that perform only database reads before rendering.

- [x] **Step 1: Remove reconciliation from the active-context list path**

Delete the `reconcileDefaultBranches(database, repos, true)` call and its error wrapper from `repoListCmd`. Leave reconciliation callers in operational commands unchanged.

- [x] **Step 2: Update command documentation**

Change `repoListCmd.Long` and the README command reference to say that listing uses GitM's cached default-branch metadata and performs no network requests. Point users to `gitm doctor` when they need metadata refreshed and checked.

- [x] **Step 3: Run the focused test to verify it passes**

Run the focused command from Task 1. Expected: PASS, with no remote marker created.

- [x] **Step 4: Run the existing repository-list tests**

Run:

```bash
GOCACHE=/tmp/gitm-repo-list-gocache GOTMPDIR=/tmp go test ./internal/cli -run '^TestRepoList' -count=1 -race -v
```

Expected: PASS.

### Task 3: Verify and deliver

**Files:**
- Verify all modified files from Tasks 1 and 2.

**Interfaces:**
- Consumes: the completed implementation and documentation.
- Produces: formatted, lint-clean, race-tested code and a pull request against `master`.

- [x] **Step 1: Format and inspect**

Run `goimports -w ./...`, `gofmt -s -w ./...`, `git diff --check`, and inspect the complete diff for unrelated changes.

- [x] **Step 2: Run lint and the full test suite**

Run `make lint` and `make test` with `GOCACHE` and `GOTMPDIR` directed under `/tmp`. Permit localhost sockets for the existing real-network integration tests.

- [x] **Step 3: Measure the original command**

Build the branch binary and time `gitm repo list` against the existing 34-repository database. Confirm the output still includes all repositories and the runtime no longer scales with remote latency.

- [ ] **Step 4: Review, commit, push, and open the pull request**

Review the diff against `master`, commit with a message explaining why listing must remain network-independent, push `codex/fix-repo-list-latency`, and create a non-draft GitHub pull request targeting `master` with the diagnosis and before/after timing.
