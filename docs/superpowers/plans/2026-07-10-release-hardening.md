# Release Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make signed upgrades fail closed, make multi-repository failures observable through exit status, make push/branch recovery safe, and restore trustworthy CI/release gates.

**Architecture:** Preserve the existing command layout and runner API. Add only a focused Git push-error classifier, then make each affected command aggregate its existing per-repository results. Keep all Git integration tests on real temporary repositories and keep upgrade HTTP seams unchanged.

**Tech Stack:** Go 1.26.2, Cobra, real Git repositories in `t.TempDir`, SQLite, golangci-lint, GitHub Actions, Make.

## Global Constraints

- Do not add external dependencies.
- Never mock Git operations; use the real `git` binary and temporary repositories.
- Every new function receives tests.
- Wrap returned errors with operation context.
- Keep per-repository interaction sequential and I/O-only multi-repository work parallel.
- Run race tests with a 180-second package timeout.
- Update README command behavior when user-visible semantics change.

---

### Task 1: Fail-closed signed self-upgrade

**Files:**
- Modify: `internal/cli/upgrade.go:376-491`
- Modify: `internal/cli/upgrade_test.go:165-691`

**Interfaces:**
- Consumes: existing `upgradeClient`, `signatureVerifier`, `findAssetURL`, and `runUpgrade`.
- Produces: `runUpgrade` behavior that requires the binary, checksum file, signature bundle, and verifier.

- [ ] **Step 1: Write failing missing-verification-asset tests**

Add table-driven tests that construct a real temporary executable through `testExecPath(t)` and fake release metadata. Each case must assert an error and assert the executable still contains `old-binary`:

```go
func TestRunUpgradeRequiresVerificationAssets(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		assets     []ghAsset
		want       string
		verifier   signatureVerifier
	}{
		{
			name: "missing checksums",
			assets: []ghAsset{{Name: name, BrowserDownloadURL: "https://example.com/binary"}},
			want: "checksums.txt",
			verifier: &fakeSignatureVerifier{},
		},
		{
			name: "missing signature bundle",
			assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
			},
			want: "checksums.txt.bundle",
			verifier: &fakeSignatureVerifier{},
		},
		{
			name: "missing verifier",
			assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
				{Name: "checksums.txt.bundle", BrowserDownloadURL: "https://example.com/bundle"},
			},
			want: "no verifier",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execPath := testExecPath(t)
			uc := &fakeUpgradeClient{
				release: &ghRelease{TagName: "v2.0.0", Assets: tt.assets},
				files: map[string][]byte{
					"https://example.com/binary":    []byte("new-binary"),
					"https://example.com/checksums": []byte("unused"),
					"https://example.com/bundle":    []byte("unused"),
				},
			}
			err := runUpgrade("v1.0.0", uc, tt.verifier, &upgradeOpts{execPath: execPath})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("runUpgrade() error = %v, want containing %q", err, tt.want)
			}
			got, readErr := os.ReadFile(execPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != "old-binary" {
				t.Fatalf("executable changed to %q", got)
			}
		})
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/cli -run 'TestRunUpgradeRequiresVerificationAssets' -count=1`

Expected: missing checksums and missing bundle cases install successfully or fail for the wrong reason.

- [ ] **Step 3: Require all trust assets before downloading**

Change `runUpgrade` to validate assets and verifier before creating/downloading the binary:

```go
checksumURL, ok := findAssetURL(rel.Assets, "checksums.txt")
if !ok {
	return fmt.Errorf("release %s is missing required asset %q", rel.TagName, "checksums.txt")
}
bundleURL, ok := findAssetURL(rel.Assets, "checksums.txt.bundle")
if !ok {
	return fmt.Errorf("release %s is missing required asset %q", rel.TagName, "checksums.txt.bundle")
}
if sv == nil {
	return fmt.Errorf("release %s cannot be verified: no verifier is available", rel.TagName)
}
```

Remove the unsigned fallback branch and make signature plus checksum verification unconditional.

- [ ] **Step 4: Update the former unsigned-fallback test**

Replace `TestRunUpgradeBundleMissingFallback` with a refusal test asserting that the executable remains unchanged.

- [ ] **Step 5: Run upgrade tests and verify GREEN**

Run: `go test ./internal/cli -run 'TestRunUpgrade|TestInstallBinary|TestSigstore' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the trust-policy change**

```bash
git add internal/cli/upgrade.go internal/cli/upgrade_test.go
git commit -m "fix: require signed assets for self-upgrade" -m "Prevents a malformed or compromised latest release from bypassing the signed-checksum trust policy."
```

### Task 2: Rebase only non-fast-forward push failures

**Files:**
- Modify: `internal/git/git.go:1-30,537-546`
- Modify: `internal/git/git_more_test.go`
- Modify: `internal/cli/push.go:166-198`
- Modify: `internal/cli/push_run_test.go`

**Interfaces:**
- Produces: `git.IsNonFastForwardError(err error) bool`.
- Consumes: `pushRepo` calls the classifier before attempting recovery.

- [ ] **Step 1: Write classifier tests**

```go
func TestIsNonFastForwardError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "fetch first", err: errors.New("! [rejected] main -> main (fetch first)"), want: true},
		{name: "non fast forward", err: errors.New("non-fast-forward"), want: true},
		{name: "authentication", err: errors.New("Authentication failed"), want: false},
		{name: "missing remote", err: errors.New("'origin' does not appear to be a git repository"), want: false},
		{name: "hook", err: errors.New("pre-push hook declined"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.IsNonFastForwardError(tt.err); got != tt.want {
				t.Fatalf("IsNonFastForwardError() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Write a real-repository missing-origin push test**

Create a repository with a local feature commit and no `origin`, call `pushRepo`, and assert the returned error contains the original push context but not `auto-rebase` or `pull`.

- [ ] **Step 3: Run focused tests and verify RED**

Run: `go test ./internal/git ./internal/cli -run 'TestIsNonFastForwardError|TestPushRepoDoesNotRebaseUnrelatedFailure' -count=1`

Expected: classifier is undefined and unrelated push failure enters recovery.

- [ ] **Step 4: Implement minimal classification and guard**

```go
func IsNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "non-fast-forward") ||
		(strings.Contains(msg, "[rejected]") && strings.Contains(msg, "fetch first"))
}
```

In `pushRepo`, return `fmt.Errorf("push %s: %w", branch, pushErr)` unless the classifier returns true.

- [ ] **Step 5: Run all push tests and verify GREEN**

Run: `go test ./internal/git ./internal/cli -run 'Push|NonFastForward' -count=1`

Expected: existing real divergence recovery still passes and unrelated failures do not rebase.

- [ ] **Step 6: Commit push classification**

```bash
git add internal/git/git.go internal/git/git_more_test.go internal/cli/push.go internal/cli/push_run_test.go
git commit -m "fix: limit push recovery to divergence" -m "Avoids changing local history after authentication, network, hook, or remote-configuration failures."
```

### Task 3: Make branch creation and rename remote-safe

**Files:**
- Modify: `internal/cli/branch_run.go:1-190`
- Modify: `internal/cli/branch_run_test.go`

**Interfaces:**
- Produces: branch-create refresh logic using existing `git.HasUpstream` and `git.Pull`.
- Produces: remote rename order `local rename -> push new -> delete old`.

- [ ] **Step 1: Write a stale-base branch-create test**

Use a real repository whose checked-out base has a configured upstream pointing at a removed/unreachable remote. Call branch creation and assert it returns an error, does not create the requested branch, and remains on the base.

- [ ] **Step 2: Write a local-only base test**

Use a repository without an upstream, create a branch with `noRemote=true`, and assert success plus the new local branch.

- [ ] **Step 3: Write remote rename ordering tests**

Use a bare remote. For the failure case, configure a rejecting `pre-receive` hook after the old branch exists, invoke rename, and assert the old remote ref still exists. For success, assert the new ref exists and old ref is absent.

- [ ] **Step 4: Run branch tests and verify RED**

Run: `go test ./internal/cli -run 'TestBranchCreate_PullFailure|TestBranchCreate_NoUpstream|TestBranchRename_' -count=1`

Expected: stale-base creation succeeds and failed publication deletes or attempts deletion of the old remote first.

- [ ] **Step 5: Implement safe base refresh**

After base checkout:

```go
baseNote := ""
hasUpstream, err := git.HasUpstream(repo.Path)
if err != nil {
	return "", "", fmt.Errorf("check upstream for %s: %w", base, err)
}
if hasUpstream {
	if _, err := git.Pull(repo.Path); err != nil {
		return "", "", fmt.Errorf("refresh base %s: %w", base, err)
	}
} else {
	baseNote = " (base has no upstream; used local state)"
}
```

Append `baseNote` to the successful result.

- [ ] **Step 6: Reorder remote rename**

Push `newName` before checking/deleting the old remote branch. Wrap deletion failure with a message stating that the new branch is published and the old remote still needs deletion.

- [ ] **Step 7: Aggregate runner failures for all branch operations**

Capture each `runner.Run` result and return `fmt.Errorf("%d repository(ies) failed to ...", runner.ErrorCount(results))` when needed.

- [ ] **Step 8: Run branch tests and verify GREEN**

Run: `go test ./internal/cli -run 'Branch' -count=1`

Expected: PASS.

- [ ] **Step 9: Commit branch safety**

```bash
git add internal/cli/branch_run.go internal/cli/branch_run_test.go
git commit -m "fix: preserve remote branches on failed updates" -m "Requires a refreshed upstream for new work and publishes renamed branches before removing their prior remote name."
```

### Task 4: Return non-zero for runner-based command failures

**Files:**
- Modify: `internal/cli/stash.go`
- Modify: `internal/cli/stash_run_test.go`
- Modify: `internal/cli/reset.go`
- Modify: `internal/cli/reset_run_test.go`

**Interfaces:**
- Changes: stash push/apply/pop and reset return aggregate operational errors.
- Changes: `offerForcePush(repos []*db.Repository, undoneCommits int) error`.

- [ ] **Step 1: Write failing stash error-return tests**

Use a registered repository path that does not exist for stash push, and a real repository with a conflicting stash pop for apply/pop. Assert returned errors include the number of failed repositories.

- [ ] **Step 2: Write reset and force-push error-return tests**

Use a broken repository for reset runner failure. For force-push, approve confirmation through a temporary stdin pipe and use a real repository whose origin rejects the push; assert `offerForcePush` returns an error.

- [ ] **Step 3: Run focused tests and verify RED**

Run: `go test ./internal/cli -run 'TestRunStash.*ReturnsError|TestRunReset.*ReturnsError|TestOfferForcePush.*ReturnsError' -count=1`

Expected: functions return nil despite printed failures, and `offerForcePush` has no return value.

- [ ] **Step 4: Aggregate stash runner results**

Capture `results := runner.Run(...)`; after output, return a contextual count error when `runner.HasErrors(results)`.

- [ ] **Step 5: Return reset and force-push errors**

Return reset runner errors before prompting. Change `offerForcePush` to return errors from confirmation reads and runner results; declining confirmation remains nil.

- [ ] **Step 6: Run stash/reset tests and verify GREEN**

Run: `go test ./internal/cli -run 'Stash|Reset|ForcePush' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit runner-based exit semantics**

```bash
git add internal/cli/stash.go internal/cli/stash_run_test.go internal/cli/reset.go internal/cli/reset_run_test.go
git commit -m "fix: surface stash and reset failures" -m "Lets scripts detect partial multi-repository failures after complete summaries are printed."
```

### Task 5: Return non-zero for sequential command failures

**Files:**
- Modify: `internal/cli/commit.go`
- Modify: `internal/cli/commit_run_test.go`
- Modify: `internal/cli/discard.go`
- Modify: `internal/cli/discard_run_test.go`
- Modify: `internal/cli/track.go`
- Modify: `internal/cli/track_test.go`
- Modify: `internal/cli/untrack.go`
- Modify: `internal/cli/untrack_test.go`

**Interfaces:**
- Changes: existing sequential workflows return `fmt.Errorf("%d repository(ies) failed to <operation>", failed)` after their summary when `failed > 0`.

- [ ] **Step 1: Add failure-return assertions to existing real-error tests**

Update commit file-selection/staging failure tests, discard invalid selection/path tests, track staging failure tests, and untrack removal failure tests to require non-nil aggregate errors after processing.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/cli -run 'TestRunCommit_(FileSelectError|StageError)|TestRunDiscard.*Error|TestRunTrack.*Error|TestRunUntrack.*Error' -count=1`

Expected: current functions return nil.

- [ ] **Step 3: Return aggregate errors after summaries**

At the end of each workflow:

```go
if failed > 0 {
	return fmt.Errorf("%d repository(ies) failed to commit", failed)
}
return nil
```

Use the correct operation verb for each command. Do not change cancellation or skip behavior.

- [ ] **Step 4: Run sequential command tests and verify GREEN**

Run: `go test ./internal/cli -run 'Commit|Discard|Track|Untrack' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit sequential exit semantics**

```bash
git add internal/cli/commit.go internal/cli/commit_run_test.go internal/cli/discard.go internal/cli/discard_run_test.go internal/cli/track.go internal/cli/track_test.go internal/cli/untrack.go internal/cli/untrack_test.go
git commit -m "fix: return failures from interactive workflows" -m "Preserves complete per-repository summaries while making automation observe unsuccessful operations."
```

### Task 6: Repair quality gates and dependency metadata

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`
- Modify: `go.mod`
- Modify: `go.sum` only if `go mod tidy` changes it

**Interfaces:**
- Produces: `TEST_TIMEOUT := 180s` shared by test/coverage targets.
- Produces: lint targets that fail when lint fails and explain absence distinctly.

- [ ] **Step 1: Demonstrate current target failures**

Run: `make test`

Expected: FAIL because `internal/cli` exceeds 60 seconds.

Run with a temporary failing linter wrapper earlier on `PATH` and execute `make lint-check`.

Expected: exit 0 with the misleading “not installed” message.

- [ ] **Step 2: Fix Make targets**

Add `TEST_TIMEOUT := 180s`, use it in test and coverage commands, replace the `which && lint || echo` expressions with explicit `command -v` conditionals that exit 1 when unavailable, and replace the stale `github.com/anomalyco` coverage prefix with `github.com/alexandreafj/gitm`.

- [ ] **Step 3: Pin and install the same linter in CI and release**

Define `GOLANGCI_LINT_VERSION: v1.64.8` at workflow scope and run:

```yaml
- name: Install golangci-lint
  run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
```

Call `make lint-check` in both workflows.

- [ ] **Step 4: Tidy module metadata**

Run: `go mod tidy`

Expected: `github.com/sigstore/protobuf-specs v0.5.0` moves from indirect to direct and no dependency versions change.

- [ ] **Step 5: Verify quality targets**

Run: `make format-check`

Run: `make lint-check`

Run: `make test`

Run: `make coverage-check`

Expected: all exit 0; test and coverage no longer time out.

- [ ] **Step 6: Commit quality-gate repairs**

```bash
git add Makefile .github/workflows/ci.yml .github/workflows/release.yml go.mod go.sum
git commit -m "ci: make release quality gates authoritative" -m "Prevents timeout and lint-wrapper behavior from hiding regressions during CI and signed releases."
```

### Task 7: Synchronize user and contributor documentation

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `internal/cli/upgrade.go:519-551`

**Interfaces:**
- Documents: mandatory signed upgrade assets, failure exit behavior, safe branch refresh/rename order, 180-second test timeout, and current direct dependencies.

- [ ] **Step 1: Update upgrade help and README**

Remove unsigned fallback wording. State that missing verification assets abort the update. Add the signature verification line to the command-reference example.

- [ ] **Step 2: Update affected command behavior**

Document that operational per-repository failures make the command exit non-zero after all repositories finish. Document branch-create no-upstream behavior and publish-before-delete rename ordering.

- [ ] **Step 3: Update testing and dependency metadata**

Change documented test timeout to 180 seconds, refresh test counts from `rg`, include both Sigstore direct dependencies, and update the approved direct-dependency count in `AGENTS.md` without changing the no-new-dependency rule.

- [ ] **Step 4: Verify help and documentation formatting**

Run: `go test ./internal/e2e -run 'TestHelp' -count=1`

Run: `git diff --check`

Expected: PASS and no whitespace errors.

- [ ] **Step 5: Commit documentation synchronization**

```bash
git add README.md AGENTS.md internal/cli/upgrade.go
git commit -m "docs: align hardening behavior and quality guidance" -m "Keeps upgrade trust, failure semantics, test commands, and approved dependency documentation consistent with the implementation."
```

### Task 8: Final verification, review, and PR

**Files:**
- Review: every file changed on `codex/fix-release-hardening`

**Interfaces:**
- Produces: a verified commit series and GitHub pull request.

- [ ] **Step 1: Format implementation files**

Run: `goimports -w ./internal ./cmd`

Run: `gofmt -s -w ./internal ./cmd`

- [ ] **Step 2: Run complete verification**

Run: `make format-check`

Run: `make lint-check`

Run: `make test`

Run: `make coverage-check`

Run: `go mod tidy -diff`

Run: `go build ./cmd/gitm`

Expected: every command exits 0.

- [ ] **Step 3: Review the complete diff**

Run: `git diff --check master...HEAD`

Run: `git diff --stat master...HEAD`

Run: `git log --oneline master..HEAD`

Confirm the branch contains only the release-hardening commits and no generated binaries or unrelated files.

- [ ] **Step 4: Push the feature branch**

Run: `git push -u origin codex/fix-release-hardening`

Expected: remote branch created successfully.

- [ ] **Step 5: Open a ready-for-review PR**

Run `gh pr create` with a title explaining release hardening and a body containing the security changes, command semantics, branch safety, quality-gate repairs, and exact verification commands.

Expected: GitHub returns the new PR URL.
