# Checkout Dirty Current Branch Design

## Problem

`gitm checkout <branch>` checks out the requested branch and then pulls it. When a repository is already on that branch with tracked uncommitted changes, the checkout is a no-op but the pull still runs. Git configurations that rebase on pull reject the dirty worktree, so GitM reports an error such as `cannot pull with rebase: You have unstaged changes`.

This is not a checkout failure. The user is already working on the requested branch, and GitM must preserve that work and report the repository as skipped.

## Required Behavior

- When the requested branch is already current and tracked uncommitted changes exist, GitM skips that repository before checkout or pull.
- The skip message states that the repository is already on the branch with uncommitted changes and that the pull was skipped.
- When the requested branch is already current and the worktree is clean, GitM still pulls. The remote may contain commits created or merged through GitHub.
- When the requested branch differs from the current branch, existing checkout behavior remains unchanged. Git may carry non-conflicting local changes to the target branch or reject conflicting changes, as it does today.
- Untracked-only files do not trigger the new skip because checkout and pull currently use the tracked-only dirty policy.
- Explicit branch, interactive, default-branch, and dry-run modes follow the same rule.

## Design

Add a small checkout preflight helper in `internal/cli/checkout.go`. It reads the current branch with `git.CurrentBranch`. Only when that branch equals the requested target does it inspect tracked changes with `git.IsDirtyTrackedOnly`.

The helper returns a skip reason when both conditions are true. Callers propagate inspection failures with operation context instead of suppressing them.

The default-branch runner and the shared explicit/interactive repository operation call the helper before `git checkout`. A skip reason is returned through the runner's existing `(message, skipReason, error)` contract, producing `SKIPPED` output and a successful overall command when no repositories fail.

Dry-run builders use the same preflight. A dirty repository already on the target is displayed as a known skip instead of listing checkout and pull actions. Clean repositories retain the pull action.

## Output

For a repository already on `AA-19432` with tracked uncommitted work:

```text
[repository          ] ⚠ SKIPPED: already on AA-19432 with uncommitted changes — pull skipped
```

For the same repository when clean, checkout continues to report the pull result, including downloaded changes or `already up to date`.

## Error Handling

- Failure to determine the current branch is returned with `current branch` context.
- Failure to inspect tracked changes is returned with `inspect tracked changes` context.
- Existing checkout conflict, missing branch, missing upstream, fetch, and pull handling remains unchanged.

## Testing

Tests use real temporary Git repositories and the actual `git` binary.

- Add a regression test with a tracked unstaged change on the already-current branch and repository-local `pull.rebase=true`. It must return a skip reason, preserve the file, and avoid an error.
- Add a clean same-branch test where another clone advances the remote. GitM must pull and make the remote change visible locally.
- Cover default-branch mode so it uses the same dirty-current-branch rule.
- Cover dry-run output for the known skip.
- Run formatting, linting, and the full race-enabled test suite before committing the implementation.

## Documentation

Update the checkout command's long help and the README command reference to document both sides of the rule: dirty same-branch repositories are skipped, while clean same-branch repositories are still pulled.
