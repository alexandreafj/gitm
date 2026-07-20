# Branch Delete Automatic Checkout Design

## Goal

When `gitm branch delete` targets the branch currently checked out in a
repository, automatically switch that repository to its configured default
branch and continue deleting the requested branch. Users should not need to run
a separate checkout command first.

## Behavior

The delete command keeps its existing selection, confirmation, parallel
execution, default-branch protection, merge-safety, force-delete, and remote
deletion behavior.

For each selected repository:

1. Refuse to delete the repository's configured default branch.
2. Determine the currently checked-out branch.
3. If the requested branch is current, check out the configured default branch
   without pulling it.
4. Delete the requested local branch when it exists.
5. Delete the matching remote branch when it exists and `--no-remote` is not
   set.

The success result will state that the repository switched to its default
branch before reporting which refs were deleted.

## Failure Handling

If checkout fails, the repository operation returns a contextual error and does
not attempt either local or remote deletion. This protects the branch when
uncommitted changes or another Git condition prevents switching safely. Other
selected repositories continue independently through the existing parallel
runner, and the command returns a non-zero aggregate result when any repository
fails.

The default branch remains protected even if it is also the current branch.
No pull is attempted during automatic checkout, so branch deletion cannot fail
because of an unrelated network or upstream problem.

## Dry Run

When the requested branch is currently checked out, `--dry-run` will show
`git checkout <default-branch>` before the existing local and remote deletion
commands. It will make no changes and will not classify the current branch as a
skip.

## Documentation

The command's long help and README command reference will explain automatic
checkout, checkout-only behavior, failure safety, and dry-run output. Examples
and repository aliases remain generic.

## Testing

Tests will use real temporary Git repositories and the actual Git binary. They
will cover:

- switching from a checked-out feature branch to a configured `main` branch and
  deleting the feature branch;
- switching to a configured `master` branch;
- preserving the branch when checkout fails;
- showing checkout before deletion in dry-run output without changing state;
- retaining default-branch deletion protection.

Final verification will run formatting, lint, and the complete race-enabled test
suite required by the project.
