# Checkout Dirty Current Branch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `gitm checkout` skip an already-current branch with tracked uncommitted work while preserving automatic pulls for an already-current clean branch.

**Architecture:** Add one CLI-layer preflight helper that compares the requested branch with Git's current branch and inspects tracked changes only when they match. Use the helper in explicit/interactive, default-branch, and dry-run checkout paths; leave different-branch checkout behavior unchanged.

**Tech Stack:** Go, Cobra CLI, the existing `internal/git` real-Git helpers, the existing parallel runner, and Go's standard `testing` package.

## Global Constraints

- Use real temporary Git repositories and the actual `git` binary; never mock Git operations.
- Add no dependencies.
- Pass state explicitly and introduce no global state.
- Wrap every new error with operation context.
- Every new function must have tests.
- Preserve different-branch checkout behavior, including carrying non-conflicting tracked changes and skipping conflicts.
- Ignore untracked-only files under the tracked-only checkout policy.
- Keep CLI help and README behavior documentation aligned.
- Run `goimports -w ./...`, `gofmt -s -w ./...`, `make lint`, and `make test` before the implementation commit.

---

### Task 1: Guard same-branch pulls and document the behavior

**Files:**
- Modify: `internal/cli/checkout.go`
- Modify: `internal/cli/checkout_run_test.go`
- Modify: `internal/e2e/checkout_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `git.CurrentBranch(path string) (string, error)`, `git.IsDirtyTrackedOnly(path string) (bool, error)`, and the runner operation contract `(message string, skipReason string, err error)`.
- Produces: `dirtyCurrentBranchSkip(path, target string) (string, error)`, returning an empty string when checkout may continue or a user-facing skip reason when the target is already current and has tracked changes.

- [ ] **Step 1: Add the failing explicit-branch regression test**

Add this real-repository test beside the existing `checkoutBranchInRepo` tests in `internal/cli/checkout_run_test.go`:

```go
func TestCheckoutBranchInRepo_DirtyCurrentBranchSkipsPull(t *testing.T) {
	dir, _, _ := initRepoWithRemote(t)

	writeFile(t, dir, "work.txt", "committed\n")
	mustRunGit(t, dir, "add", "work.txt")
	mustRunGit(t, dir, "commit", "-m", "add work file")
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "AA-19432")
	mustRunGit(t, dir, "config", "pull.rebase", "true")
	writeFile(t, dir, "work.txt", "uncommitted\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "AA-19432")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if message != "" {
		t.Fatalf("message = %q, want empty", message)
	}
	want := "already on AA-19432 with uncommitted changes — pull skipped"
	if skipReason != want {
		t.Fatalf("skip reason = %q, want %q", skipReason, want)
	}
	content, err := os.ReadFile(filepath.Join(dir, "work.txt"))
	if err != nil {
		t.Fatalf("read work.txt: %v", err)
	}
	if got := string(content); got != "uncommitted\n" {
		t.Fatalf("work.txt = %q, want uncommitted work preserved", got)
	}
}
```

- [ ] **Step 2: Run the regression test and verify RED**

Run:

```bash
go test ./internal/cli -run '^TestCheckoutBranchInRepo_DirtyCurrentBranchSkipsPull$' -v -count=1
```

Expected: FAIL because the current implementation reaches `git pull --ff-only`, and repository-local `pull.rebase=true` rejects the unstaged change.

- [ ] **Step 3: Add the minimal preflight helper and explicit/interactive call**

Add this helper near `checkoutBranchInRepo` in `internal/cli/checkout.go`:

```go
func dirtyCurrentBranchSkip(path, target string) (string, error) {
	current, err := git.CurrentBranch(path)
	if err != nil {
		return "", fmt.Errorf("current branch: %w", err)
	}
	if current != target {
		return "", nil
	}

	dirty, err := git.IsDirtyTrackedOnly(path)
	if err != nil {
		return "", fmt.Errorf("inspect tracked changes: %w", err)
	}
	if !dirty {
		return "", nil
	}

	return fmt.Sprintf("already on %s with uncommitted changes — pull skipped", target), nil
}
```

After branch existence and any remote-only fetch in `checkoutBranchInRepo`, add:

```go
	skipReason, err := dirtyCurrentBranchSkip(repo.Path, branch)
	if err != nil {
		return "", "", err
	}
	if skipReason != "" {
		return "", skipReason, nil
	}
```

- [ ] **Step 4: Run the explicit regression test and verify GREEN**

Run:

```bash
go test ./internal/cli -run '^TestCheckoutBranchInRepo_DirtyCurrentBranchSkipsPull$' -v -count=1
```

Expected: PASS with the dirty file preserved.

- [ ] **Step 5: Add a clean same-branch pull preservation test**

Add this characterization test in `internal/cli/checkout_run_test.go`:

```go
func TestCheckoutBranchInRepo_CleanCurrentBranchPullsRemoteChanges(t *testing.T) {
	dir, origin, _ := initRepoWithRemote(t)
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "AA-19432")

	other := filepath.Join(t.TempDir(), "other")
	mustRunGit(t, t.TempDir(), "clone", origin, other)
	mustRunGit(t, other, "checkout", "AA-19432")
	writeFile(t, other, "remote.txt", "from GitHub\n")
	mustRunGit(t, other, "add", "remote.txt")
	mustRunGit(t, other, "commit", "-m", "advance remote branch")
	mustRunGit(t, other, "push")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "AA-19432")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if skipReason != "" {
		t.Fatalf("skip reason = %q, want empty", skipReason)
	}
	if message == "" {
		t.Fatal("expected pull result message")
	}
	content, err := os.ReadFile(filepath.Join(dir, "remote.txt"))
	if err != nil {
		t.Fatalf("read remote.txt: %v", err)
	}
	if got := string(content); got != "from GitHub\n" {
		t.Fatalf("remote.txt = %q, want pulled remote content", got)
	}
}
```

Run:

```bash
go test ./internal/cli -run '^TestCheckoutBranchInRepo_(DirtyCurrentBranchSkipsPull|CleanCurrentBranchPullsRemoteChanges)$' -v -count=1
```

Expected: PASS. This protects the requirement that clean current branches still pull.

- [ ] **Step 6: Add the failing default-branch regression test**

Add this test in `internal/cli/checkout_run_test.go`:

```go
func TestCheckoutDefault_DirtyCurrentBranchSkipsPull(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	writeFile(t, dir, "work.txt", "committed\n")
	mustRunGit(t, dir, "add", "work.txt")
	mustRunGit(t, dir, "commit", "-m", "add work file")
	mustRunGit(t, dir, "push")
	mustRunGit(t, dir, "config", "pull.rebase", "true")
	writeFile(t, dir, "work.txt", "uncommitted\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
			t.Fatalf("runCheckoutDefault: %v", err)
		}
	})
	if !strings.Contains(output, "SKIPPED: already on main with uncommitted changes — pull skipped") {
		t.Fatalf("output does not contain dirty current-branch skip:\n%s", output)
	}
}
```

Run:

```bash
go test ./internal/cli -run '^TestCheckoutDefault_DirtyCurrentBranchSkipsPull$' -v -count=1
```

Expected: FAIL because default checkout still reaches the pull.

- [ ] **Step 7: Apply the preflight to default checkout and verify GREEN**

At the beginning of the `runner.Run` closure in `runCheckoutDefaultDryRun`, add:

```go
		skipReason, err := dirtyCurrentBranchSkip(repo.Path, repo.DefaultBranch)
		if err != nil {
			return "", "", err
		}
		if skipReason != "" {
			return "", skipReason, nil
		}
```

Run:

```bash
go test ./internal/cli -run '^TestCheckout(Default_DirtyCurrentBranchSkipsPull|BranchInRepo_DirtyCurrentBranchSkipsPull|BranchInRepo_CleanCurrentBranchPullsRemoteChanges)$' -v -count=1
```

Expected: PASS.

- [ ] **Step 8: Add failing dry-run coverage**

Add this test in `internal/cli/checkout_run_test.go`:

```go
func TestCheckoutBranchDryRun_DirtyCurrentBranchShowsSkip(t *testing.T) {
	dir := initRepo(t)
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	writeFile(t, dir, "README.md", "uncommitted\n")
	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}

	items := checkoutBranchDryRunItems([]*db.Repository{repo}, "AA-19432")
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	want := "already on AA-19432 with uncommitted changes — pull skipped"
	if items[0].skipReason != want {
		t.Fatalf("skip reason = %q, want %q", items[0].skipReason, want)
	}
	if len(items[0].actions) != 0 {
		t.Fatalf("actions = %v, want none for known skip", items[0].actions)
	}
}

func TestCheckoutDefaultDryRun_DirtyCurrentBranchShowsSkip(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "uncommitted\n")
	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}

	items := checkoutDefaultDryRunItems([]*db.Repository{repo})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	want := "already on main with uncommitted changes — pull skipped"
	if items[0].skipReason != want {
		t.Fatalf("skip reason = %q, want %q", items[0].skipReason, want)
	}
	if len(items[0].actions) != 0 {
		t.Fatalf("actions = %v, want none for known skip", items[0].actions)
	}
}
```

Run:

```bash
go test ./internal/cli -run '^TestCheckout(Branch|Default)DryRun_DirtyCurrentBranchShowsSkip$' -v -count=1
```

Expected: FAIL because both dry-run builders still list checkout and pull actions.

- [ ] **Step 9: Apply the preflight to both dry-run builders and verify GREEN**

At the beginning of each builder loop, after constructing `item`, add the equivalent target-specific block:

```go
		skipReason, stateErr := dirtyCurrentBranchSkip(repo.Path, target)
		if skipReason != "" {
			item.skipReason = skipReason
			items = append(items, item)
			continue
		}
```

Use `repo.DefaultBranch` as `target` in `checkoutDefaultDryRunItems` and `branch` in `checkoutBranchDryRunItems`. When `stateErr != nil`, retain the planned actions and set this warning instead of suppressing the inspection failure:

```go
		item.warning = fmt.Sprintf("could not inspect current branch state: %v", stateErr)
```

Only run the existing tracked-change warning logic when `stateErr == nil`.

Run:

```bash
go test ./internal/cli -run '^TestCheckout(Branch|Default)DryRun_DirtyCurrentBranchShowsSkip$' -v -count=1
```

Expected: PASS.

- [ ] **Step 10: Add CLI-level regression coverage**

Add this real end-to-end test in `internal/e2e/checkout_test.go`:

```go
func TestCheckout_DirtyCurrentBranchSkipsPull(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-dirty-current")
	e.runGitm("repo", "add", repo, "--alias", "co-dirty-current")
	e.mustGit(repo, "checkout", "-b", "AA-19432")
	e.mustGit(repo, "push", "--set-upstream", "origin", "AA-19432")
	e.mustGit(repo, "config", "pull.rebase", "true")
	e.writeFile(repo, "README.md", "uncommitted work\n")

	r := e.runGitm("checkout", "AA-19432", "--repo", "co-dirty-current")
	e.assertExitCode(r, 0)
	e.assertContains(r, "SKIPPED: already on AA-19432 with uncommitted changes — pull skipped")
	content, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if got := string(content); got != "uncommitted work\n" {
		t.Fatalf("README.md = %q, want uncommitted work preserved", got)
	}
}
```

Run:

```bash
go test ./internal/e2e -run '^TestCheckout_DirtyCurrentBranchSkipsPull$' -v -count=1
```

Expected: PASS against the implemented preflight and the real CLI binary.

- [ ] **Step 11: Update help and README documentation**

In the checkout command's `Long` text, add:

```text
When the requested branch is already current, tracked uncommitted changes skip
the checkout and pull. A clean current branch still pulls remote updates.
```

Replace the README's broad tracked-change behavior bullet with:

```markdown
- When the requested branch is already current, tracked uncommitted changes skip checkout and pull; a clean current branch still pulls remote updates. When switching branches, Git carries non-conflicting changes and skips conflicts. Untracked-only files are ignored.
```

Update the checkout example skip message to:

```text
[frontend           ] ⚠ SKIPPED: already on main with uncommitted changes — pull skipped
```

- [ ] **Step 12: Format and run focused verification**

Run:

```bash
goimports -w ./...
gofmt -s -w ./...
go test ./internal/cli ./internal/e2e -run 'Checkout' -v -race -count=1 -timeout 180s
git diff --check
```

Expected: all checkout tests pass under race detection and `git diff --check` prints nothing.

- [ ] **Step 13: Run repository-wide verification**

Run:

```bash
make lint
make test
```

Expected: lint exits zero and `go test ./... -v -race -timeout 180s` passes.

- [ ] **Step 14: Review and commit the implementation**

Review:

```bash
git status --short
git diff --stat
git diff
git log --oneline master..HEAD
```

Confirm the diff contains only the plan, checkout implementation, real-Git tests, and checkout documentation. Then commit:

```bash
git add docs/superpowers/plans/2026-08-11-checkout-dirty-current-branch.md internal/cli/checkout.go internal/cli/checkout_run_test.go internal/e2e/checkout_test.go README.md
git commit -m "Skip dirty pulls on the current checkout branch" -m "Preserves in-progress work when gitm checkout targets the branch already in use, while keeping automatic pulls for clean branches that may have advanced remotely."
```

- [ ] **Step 15: Push and open the pull request**

After a fresh verification on the committed tree:

```bash
git push -u origin codex/fix-checkout-dirty-current-branch
gh pr create --base master --head codex/fix-checkout-dirty-current-branch --title "Skip dirty pulls on the current checkout branch" --body-file <prepared-pr-body-file>
```

The PR body must summarize the behavior, explain why clean branches still pull, and list focused plus full verification commands.
