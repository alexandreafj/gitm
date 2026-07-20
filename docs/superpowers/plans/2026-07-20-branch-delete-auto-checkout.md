# Branch Delete Automatic Checkout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `gitm branch delete` switch away from the active target branch to each repository's configured default branch before deleting it.

**Architecture:** Extend the existing per-repository delete worker in `internal/cli/branch_run.go`. The worker will use the existing `git.Checkout` wrapper, stop before deletion when checkout fails, and include the switch in success and dry-run output; no new dependency or global state is needed.

**Tech Stack:** Go, Cobra, the existing Git wrapper and parallel runner, real temporary Git repositories in `testing`.

## Global Constraints

- Never use mocks for Git operations; use real repositories in `t.TempDir()` and the actual `git` binary.
- Add no dependencies.
- Wrap every operational error with context.
- Keep repository names, examples, and descriptions generic.
- Automatic checkout must not pull or access the network.
- Run `goimports -w ./...`, `gofmt -s -w ./...`, `make lint`, and `make test` before the final commit.

---

### Task 1: Automatic checkout and deletion

**Files:**
- Modify: `internal/cli/branch_run_test.go`
- Modify: `internal/cli/branch_run.go`

**Interfaces:**
- Consumes: `git.CurrentBranch(path string) (string, error)`, `git.Checkout(path, branch string) error`, `db.Repository.DefaultBranch string`.
- Produces: changed `runBranchDeleteWithUI` behavior; no new exported interface.

- [ ] **Step 1: Write failing real-repository tests**

Add table-driven coverage for configured `main` and `master` defaults. In each case create the default branch and a checked-out `feature/current`, invoke branch delete with `--no-remote`, then assert `git.CurrentBranch(repoDir)` is the configured default and `git.BranchExists(repoDir, "feature/current")` is false.

Add a checkout-failure case by creating conflicting uncommitted content on the feature branch and different committed content on the default branch. Assert the command returns an error, the current branch remains `feature/current`, and the branch still exists.

- [ ] **Step 2: Run focused tests and verify the behavior is missing**

Run:

```bash
go test ./internal/cli -run 'TestBranchDelete_(CurrentBranch|Checkout)' -v -count=1
```

Expected: the automatic-checkout cases fail because the branch remains checked out and undeleted; the checkout-failure case initially fails because the command reports a skip instead of an operational error.

- [ ] **Step 3: Implement minimal automatic checkout**

Replace the current checked-out-branch skip with:

```go
switchedTo := ""
if current == branchName {
	if err := git.Checkout(repo.Path, repo.DefaultBranch); err != nil {
		return "", "", fmt.Errorf("checkout default branch %s: %w", repo.DefaultBranch, err)
	}
	switchedTo = repo.DefaultBranch
}
```

After deletion, build the result from the existing deleted refs and prefix it when `switchedTo` is non-empty, for example `switched to main — deleted feature/current (local)`.

- [ ] **Step 4: Run focused tests and verify they pass**

Run the focused command from Step 2. Expected: PASS for both configured defaults and checkout-failure safety.

### Task 2: Dry-run behavior and user documentation

**Files:**
- Modify: `internal/cli/branch_run_test.go`
- Modify: `internal/cli/branch_run.go`
- Modify: `internal/cli/branch.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `branchDeleteDryRunItems`, `dryRunItem.actions`.
- Produces: dry-run action ordering and updated command documentation.

- [ ] **Step 1: Write a failing dry-run test**

Create a real repository on `feature/current`, run `runBranchDeleteWithUIDryRun`, and assert output contains these actions in order:

```text
git checkout main
git branch -d feature/current
```

Also assert the current branch and target branch still exist unchanged.

- [ ] **Step 2: Run the dry-run test and verify it fails**

Run:

```bash
go test ./internal/cli -run 'TestBranchDelete_DryRunCurrentBranch' -v -count=1
```

Expected: FAIL because dry-run currently reports `switch away first` and omits checkout/delete actions.

- [ ] **Step 3: Implement dry-run checkout preview**

When `current == branchName`, append this action and continue existing deletion analysis:

```go
item.actions = append(item.actions, fmt.Sprintf("git checkout %s", repo.DefaultBranch))
```

Update `branchDeleteCmd` long help and README behavior/safety sections to state that an active target is checked out to the configured default branch without pulling, checkout failure prevents deletion, and dry-run previews the checkout.

- [ ] **Step 4: Run focused branch tests**

Run:

```bash
go test ./internal/cli -run 'TestBranchDelete' -v -count=1
```

Expected: PASS.

### Task 3: Full verification and delivery

**Files:**
- Review all files changed from `master`.

**Interfaces:**
- Produces: formatted, linted, race-tested commit and GitHub pull request.

- [ ] **Step 1: Format the repository**

Run:

```bash
goimports -w ./...
gofmt -s -w ./...
```

Expected: commands exit zero.

- [ ] **Step 2: Run required quality gates**

Run:

```bash
make lint
make test
```

Expected: both commands exit zero; `make test` runs the complete race-enabled suite.

- [ ] **Step 3: Review the diff and forbidden project-specific text**

Run:

```bash
git diff --check
git diff master...HEAD
```

Expected: no whitespace errors and only scoped, generic changes.

- [ ] **Step 4: Commit, push, and open the PR**

Commit with a message explaining that automatic checkout removes the manual prerequisite while preserving checkout safety. Push `fix/branch-delete-auto-checkout`, then create a ready pull request against `master` using `gh pr create` with a generic title and body containing the behavior and verification results.
