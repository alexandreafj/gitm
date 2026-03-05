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
