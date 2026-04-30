# AGENTS.md

## The Landmines

### 1. **Never use mocks for git operations. Ever.**

When testing code that wraps git commands, create **real temporary repositories** in `t.TempDir()` and use the actual `git` binary to set up state.

**Why:** Mocks don't catch integration failures. A mock might pass, but the real git command could fail silently in production. Use real repos.

```go
// GOOD: real temp repo
func TestResetSoft(t *testing.T) {
    dir := initRepo(t)  
    makeCommit(t, dir, "file.go", "content", "message")
    git.ResetSoft(dir, "HEAD~1")
    staged := stagedFiles(t, dir)
    // assertions...
}

// BAD: never do this
mockGit := &MockGit{}
mockGit.On("ResetSoft", ...).Return(nil)
```

### 2. **Dependencies are locked. Adding new ones requires discussion.**

This project has exactly 5 external dependencies:
- `github.com/spf13/cobra` — CLI framework
- `modernc.org/sqlite` — SQLite (pure Go, no CGo)
- `github.com/charmbracelet/bubbletea` — TUI
- `github.com/fatih/color` — Colored output
- `golang.org/x/sync` — `errgroup`

Before adding anything else, ask: *Can this be done with stdlib?* If no, get explicit approval.

### 3. **No global state. Ever.**

Database connections, config, and loggers must be **passed explicitly** via Dependency Injection.

**Why:** Race conditions and hidden dependencies. See `internal/cli/root.go` for the pattern: load config/DB in `PersistentPreRunE`, inject into command logic, close in `PersistentPostRun`.

### 4. **Every new function gets tests. Full stop.**

No exceptions. Tests must use the real tools (git, filesystem, etc.), not mocks. Run with race detection:
```bash
go test ./... -v -race -timeout 60s
```

### 5. **Error handling: never suppress, always wrap with context.**

```go
// GOOD
if err := git.Push(repo.Path); err != nil {
    return fmt.Errorf("push failed for %s: %w", repo.Alias, err)
}

// BAD
_ = git.Push(repo.Path)
```

### 6. **Parallelism decisions:**

- **Parallel (runner.Run):** I/O-bound ops like running git across many repos
- **Sequential:** Per-repo user interaction (e.g., prompts, file selection)

Look at `internal/cli/commit.go` — it does sequential per-repo workflows while still being fast.

### 7. **Always format and lint before committing.**

```bash
goimports -w ./...
gofmt -s -w ./...
make lint
make test
```

CI will reject unformatted code.

### 8. **Commit messages explain the WHY, not the WHAT.**

```
Good:  Add git reset command with soft/mixed/hard modes
       Allows users to undo N commits with safety checks for pushed commits.

Bad:   Update files
       Add reset
```

### 9. **CLI help text must be thorough.**

Every command gets a detailed `Long` explanation with examples. Users won't read code; the help text is their only guide.

---

## Development Workflow

### 1. **Always create a feature branch from `master` before starting work.**

Never commit directly to `master`, and never branch off another in-flight feature branch — always start from a clean `master`. Otherwise the unmerged commits from the parent branch will leak into your PR's diff and bloat the review.

```bash
# Always do this first
git checkout master
git pull --ff-only

# Then branch off with a descriptive name
git checkout -b feat/add-upgrade-command
git checkout -b fix/checkout-dirty-repo-crash
git checkout -b docs/update-readme-for-stash
```

Quick self-check before the first commit: `git log --oneline master..HEAD` should be empty (or contain only commits you have explicitly authored on this branch). If it lists commits from another feature branch, you branched off the wrong place — reset to `origin/master` and cherry-pick your work.

### 2. **Every new command or subcommand must update `README.md`.**

When you add or modify a CLI command:
1. Add it to the **Table of Contents**.
2. Add a full **Commands Reference** section following the existing format: description, usage, flags table, behaviour notes, examples, and example output.
3. Update the **Project Structure** tree if new files were added.
4. Update the **"Adding a new command"** checklist if the process changed.

### 3. **Every new command needs unit tests.**

This is non-negotiable (see Landmine #4). At minimum:
- Test that the command is registered as a subcommand of root.
- Test flag parsing and validation.
- Test the core logic with real git repos (not mocks).
- Run `make test` and `make lint` before committing.

### 4. **If the command doesn't need DB access, skip initialization.**

Add the command name to the skip list in `root.go`'s `PersistentPreRunE`:
```go
if cmd.Name() == "__complete" || cmd.Name() == "help" || cmd.Name() == "upgrade" {
    return nil
}
```

---

## Rules That Never Change

1. Project structure: `/cmd/gitm`, `/internal/*`, `/pkg/` (sparingly)
2. Linting with `golangci-lint` in CI
3. Tests run with race detection
4. No hardcoded secrets
5. Use `exec.Command("git", ...)` — never shell interpolation

---

## Debugging

If something feels wrong:
1. Check if you're using the real git, not a mock
2. Verify error context is wrapped (not silently ignored)
3. Run `make test -race` locally before pushing
4. If adding a dependency, verify it's justified in the approved list

---

## Questions?

Refer to the actual code. Look at how `internal/cli/commit.go`, `internal/cli/discard.go`, and `internal/cli/reset.go` are structured — they're the canonical patterns. Copy them.

If you find yourself writing something that doesn't fit these patterns, stop and ask.
