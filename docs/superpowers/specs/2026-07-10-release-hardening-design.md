# Release Hardening Design

## Goal

Harden gitm's release and multi-repository failure behavior so upgrades never
install unverified binaries, automated callers receive trustworthy exit codes,
branch operations avoid preventable remote damage, and the documented quality
gates reliably enforce the project standards.

## Scope

This change covers the critical and high-priority findings from the 2026-07-10
codebase audit:

1. Require the binary, `checksums.txt`, and `checksums.txt.bundle` before a
   self-upgrade can install anything.
2. Rebase after a failed push only when Git positively reports a
   non-fast-forward rejection.
3. Return a non-zero command result when one or more selected repositories
   encounter operational errors.
4. Refuse to create a branch from a stale base when a configured upstream pull
   fails, while continuing to support local-only bases with no upstream.
5. Publish a renamed remote branch before deleting its old remote name.
6. Repair test, coverage, lint, dependency, CI, and release-gate drift that can
   currently hide failures.
7. Update user documentation for behavior changed by this work.

The feature-roadmap ideas and medium-priority architectural findings are not in
scope. In particular, this change will not introduce full dependency injection,
Git command contexts/timeouts, transactional migration redesign, JSON output,
workspace sessions, an undo journal, or the porcelain parser replacement.

## Upgrade Trust Policy

`gitm upgrade` will treat release verification assets as mandatory. Before
downloading or installing the binary, it will validate that the release exposes
all three expected assets for the current platform:

- the platform binary;
- `checksums.txt`;
- `checksums.txt.bundle`.

The command will fail closed if an asset is absent, the signature verifier is
unavailable, the bundle cannot be downloaded, signature verification fails, the
binary has no checksum entry, or its checksum differs. The latest-release flow
will no longer infer that an arbitrary bundle-less release "predates signing".
Legacy bundle-format parsing remains supported because signed historical bundles
use it; unsigned fallback installation does not.

## Push Failure Classification

The Git package will expose a small pure classifier for push errors. It will
recognize the C-locale markers emitted for non-fast-forward/fetch-first
rejections and return false for unrelated failures. `pushRepo` will invoke
`pull --rebase --autostash` only when that classifier returns true.

Authentication, authorization, network, missing-remote, protected-branch, hook,
and unknown failures will be returned with their original push context and will
not trigger a pull or rebase. Existing successful diverged-remote recovery and
conflict reporting remain unchanged.

## Multi-Repository Error Semantics

Commands will continue processing all selected repositories and printing the
complete summary. After processing, they will return an aggregate error when at
least one repository has an operational failure.

Intentional skips remain successful outcomes, including dirty-repository safety
skips, missing branches, protected/default branches, user cancellation, empty
selection, and deliberately declining a force-push confirmation. Merge/rebase
conflicts that the command intentionally leaves for manual resolution retain
their currently documented skip semantics.

Runner-based commands will inspect `runner.Run` results as `checkout`, `update`,
`sync`, and `push` already do. Sequential commands will count their existing
result errors and return a contextual aggregate error after printing the
summary. Reset's force-push prompt will return an error when an approved
force-push fails or confirmation input cannot be read.

## Safe Branch Creation

After checking out the base branch, branch creation will inspect whether that
base has an upstream:

- With an upstream, `git pull --ff-only` must succeed before the new branch is
  created. A failed refresh is a repository error.
- Without an upstream, the command may create the branch from the local base and
  will state that the base was not refreshed.
- Failure to determine upstream state is an error rather than permission to
  continue on uncertain state.

This preserves valid local-only repositories while preventing a network,
authentication, or remote failure from producing a successful-looking branch
based on stale state.

## Safe Remote Rename

Remote branch rename will use this order per repository:

1. Rename the local branch.
2. Push the new name and establish its upstream.
3. If the old remote branch exists, delete the old remote name.

If publishing the new name fails, the old remote branch remains intact. If old
remote deletion fails after publishing, both remote names remain and the command
returns an actionable error; the new branch remains safely published.

## Quality Gates and Metadata

The Makefile will define a shared test timeout of 180 seconds for race and
coverage runs. Lint targets will use an explicit availability check and preserve
the linter's exit code instead of converting failures into success. Coverage
package reporting will use this repository's module path.

CI and release workflows will install the same pinned golangci-lint version
before running lint. The direct Sigstore protobuf import will be classified as a
direct module dependency. Documentation dependency counts and changed command
behavior will be synchronized with the code.

## Testing Strategy

Every behavior change follows red-green-refactor development:

- Upgrade tests cover missing checksums, missing bundle, unavailable verifier,
  valid signed assets, bad signatures, and checksum mismatch.
- Push tests use real temporary remotes to preserve the existing diverged-branch
  integration coverage and add a non-divergence failure that must not invoke
  rebase recovery. The pure error classifier receives focused table tests.
- Branch-create tests cover a configured upstream pull failure and a no-upstream
  local-only success.
- Branch-rename tests prove the old remote survives a failed new-branch push and
  disappears only after the new remote ref exists.
- Each command family gains failure-exit tests using real invalid repository or
  Git state rather than mocked Git operations.
- Makefile behavior is checked by running the actual targets; the final full
  suite runs with race detection.

Final verification requires formatting, direct lint, module tidy verification,
the full race suite, the coverage threshold, a clean Git diff review, and a
successful build before commit and PR creation.

