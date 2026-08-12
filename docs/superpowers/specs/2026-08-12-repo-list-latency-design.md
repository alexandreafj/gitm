# Fast Repository Listing Design

## Problem

`gitm repo list` takes 8–12 seconds for the current 34-repository setup. The
SQLite query completes in about 0.02 seconds and uses the existing unique alias
index. The delay comes from refreshing every repository's remote default branch
with `git ls-remote --symref origin HEAD` before rendering the table. Those
network calls take roughly 1.7–2.4 seconds each and run in batches of ten.

Repository listing is an inspection command, so its latency should not depend
on network availability or the number of remote hosts involved.

## Design

`gitm repo list` will read and display repository records from SQLite without
contacting Git remotes. The `DEFAULT BRANCH` column will show the cached value
already maintained by commands that require current remote-default knowledge.

Live reconciliation remains unchanged for operational commands such as
checkout, branch protection, sync, doctor, and update fallback. Removing the
reconciliation call only from `repo list` preserves their safety behavior while
making listing deterministic and lightweight.

Both active-context listing and `gitm repo list --all` will be cache-only. The
help text and README will describe that behavior instead of promising a network
refresh.

## Error Handling

Database and context lookup failures continue to be returned with their current
context. Because listing performs no remote operations, it will no longer emit
remote-refresh warnings or wait for remote timeouts.

## Testing

Add a command test backed by a real temporary Git repository whose `origin`
points to a deliberately blocked local Git server. Execute `repo list` and
assert that it finishes before the server is released and prints the cached
default branch. This test fails while listing contacts the remote and passes
only when the command is cache-only.

Run the focused test first to demonstrate red and green states, then run
formatting, lint, the complete race-enabled test suite, and a local timing of
the built command against the existing database.
