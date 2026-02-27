# gitm — Multi-Repository Git Manager

`gitm` is a fast CLI tool for managing git operations across multiple repositories simultaneously. All operations run **in parallel** with live-streaming output, so working across 20+ repositories is as quick as working on one.

---

## Table of Contents

- [Why gitm?](#why-gitm)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands Reference](#commands-reference)
  - [repo add](#gitm-repo-add)
  - [repo list](#gitm-repo-list)
  - [repo remove](#gitm-repo-remove)
  - [checkout master](#gitm-checkout-master)
  - [branch create](#gitm-branch-create)
  - [branch rename](#gitm-branch-rename)
  - [status](#gitm-status)
  - [update](#gitm-update)
- [How It Works](#how-it-works)
- [Data Storage](#data-storage)
- [Development](#development)

---

## Why gitm?

When working across many repositories, daily git operations become repetitive:

| Without gitm | With gitm |
|---|---|
| `cd repo1 && git checkout main && git pull` × 23 repos | `gitm checkout master` |
| Manually `cd` into 6 repos to create a feature branch | `gitm branch create feature/JIRA-123` |
| Manually rename a branch in each repo + update remote | `gitm branch rename old-name new-name` |
| Forget which repos are dirty or behind origin | `gitm status` |

---

## Installation

### Prerequisites

- [Go 1.24+](https://golang.org/dl/)
- `git` available in your `PATH`

### From source

```bash
# Clone the repository
git clone https://github.com/alexandreferreira/gitm.git
cd gitm

# Build and install to GOPATH/bin
make install

# Verify installation
gitm --help
```

### Build only (without installing)

```bash
make build
# Binary will be at ./bin/gitm
./bin/gitm --help
```

### Add to PATH (if using ./bin/ directly)

```bash
export PATH="$PATH:/path/to/gitm/bin"
# Add the above to your ~/.bashrc or ~/.zshrc to make it permanent
```

---

## Quick Start

```bash
# 1. Register your repositories
gitm repo add /path/to/api-gateway
gitm repo add /path/to/auth-service /path/to/frontend /path/to/payment-svc

# Or register the current directory
cd /path/to/my-repo && gitm repo add .

# 2. See all registered repos
gitm repo list

# 3. Sync everything to the default branch (main/master) and pull
gitm checkout master

# 4. Start a new task — create a branch in selected repos interactively
gitm branch create feature/JIRA-456

# 5. Check what all your repos are doing
gitm status

# 6. Pull the latest on whatever branch each repo is currently on
gitm update
```

---

## Commands Reference

### `gitm repo add`

Register one or more local git repositories with gitm.

```
gitm repo add <path> [path...]
```

**Arguments:**

| Argument | Description |
|---|---|
| `<path>` | Absolute or relative path to a git repository. Use `.` for the current directory. |

**Examples:**

```bash
# Add a single repo
gitm repo add /home/user/work/api-gateway

# Add multiple repos at once
gitm repo add /home/user/work/api-gateway /home/user/work/auth-service /home/user/work/frontend

# Add the current directory
gitm repo add .
```

**Behaviour:**

- Validates that the path exists and is a git repository (`git rev-parse` check).
- Auto-detects the default branch (`main` or `master`) by inspecting `origin/HEAD`.
- Stores the repository name (derived from directory name), path, and default branch in `~/.gitm/gitm.db`.
- If a repository is already registered, it prints a warning and continues without error.

---

### `gitm repo list`

List all registered repositories.

```
gitm repo list
```

**Example output:**

```
#     NAME                    DEFAULT BRANCH  PATH
1     api-gateway             main            /home/user/work/api-gateway
2     auth-service            master          /home/user/work/auth-service
3     frontend                main            /home/user/work/frontend
4     payment-svc             main            /home/user/work/payment-svc

4 repository(ies) registered.
```

---

### `gitm repo remove`

Unregister a repository by name. This only removes it from gitm's database — it does **not** delete the repository from disk.

```
gitm repo remove <name>
gitm repo rm <name>       # alias
```

**Arguments:**

| Argument | Description |
|---|---|
| `<name>` | The repository name as shown in `gitm repo list` (typically the directory name). |

**Examples:**

```bash
gitm repo remove api-gateway
gitm repo rm old-service
```

---

### `gitm checkout master`

Switch all registered repositories to their default branch and pull the latest changes. Runs in **parallel**.

```
gitm checkout master
```

**Behaviour:**

1. Checks each repository for uncommitted changes (`git status --porcelain`).
2. If a repository is **dirty** (has unstaged or staged changes), it is **skipped** with a warning. Nothing is ever force-reset.
3. For clean repositories: runs `git checkout <default-branch>` then `git pull --ff-only`.
4. Streams results live as each repo completes.
5. Shows a summary at the end (`N succeeded, N skipped, N failed`).

**Example output:**

```
Checking out default branch and pulling for 4 repositories…

[api-gateway        ] ✓ on main — already up to date
[auth-service       ] ✓ on master — 3 files changed, 47 insertions(+)
[frontend           ] ⚠ SKIPPED: uncommitted changes (2 file(s)): M src/App.tsx, M package.json
[payment-svc        ] ✓ on main — already up to date

Done: 3 succeeded, 1 skipped
```

---

### `gitm branch create`

Create a new branch in selected repositories. An interactive multi-select UI lets you choose which repositories to apply the operation to.

```
gitm branch create <branch-name> [flags]
```

**Arguments:**

| Argument | Description |
|---|---|
| `<branch-name>` | The name of the new branch to create. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--all` | `-a` | false | Skip the selection UI and apply to all registered repositories. |
| `--from` | `-f` | _(repo default branch)_ | Base branch to create from instead of the repo's default branch. |

**Interactive UI:**

When you run `gitm branch create feature/JIRA-123`, you'll see:

```
Select repositories for new branch: feature/JIRA-123
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [ ] api-gateway     /home/user/work/api-gateway
▶ [✓] auth-service    /home/user/work/auth-service
  [✓] frontend        /home/user/work/frontend
  [ ] payment-svc     /home/user/work/payment-svc

2/4 selected
```

**Keybindings:**

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Space` | Toggle selection |
| `a` | Select / deselect all |
| `Enter` | Confirm selection and proceed |
| `q` / `Esc` | Cancel |

**Behaviour:**

1. For each selected repo (in parallel):
   - Checks for uncommitted changes — skips if dirty.
   - Checks out the base branch and pulls latest.
   - Creates and checks out the new branch (`git checkout -b <branch-name>`).
   - If the branch already exists, checks it out instead of failing.
2. Streams results live.

**Examples:**

```bash
# Interactive selection
gitm branch create feature/JIRA-456

# Create in all repos without prompting
gitm branch create feature/JIRA-456 --all

# Create from a specific base branch
gitm branch create hotfix/critical-bug --from develop
```

---

### `gitm branch rename`

Rename a branch across selected repositories — both locally and on the remote. Runs in **parallel**.

```
gitm branch rename <old-name> <new-name> [flags]
```

**Arguments:**

| Argument | Description |
|---|---|
| `<old-name>` | The current branch name. |
| `<new-name>` | The new branch name. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--all` | `-a` | false | Apply to all repositories that have the old branch. |
| `--no-remote` | — | false | Only rename locally. Skip deleting the old remote branch and pushing the new one. |

**Behaviour:**

1. Filters the registered repositories to only those that have a local branch named `<old-name>`.
2. Opens the interactive multi-select UI showing only the matching repositories.
3. For each selected repo (in parallel):
   - `git branch -m <old-name> <new-name>` — renames locally.
   - `git push origin --delete <old-name>` — deletes the old remote branch (if it exists).
   - `git push --set-upstream origin <new-name>` — pushes the new branch and sets tracking.
4. Streams results live.

**Examples:**

```bash
# Interactive: rename feature/JIRA-123 to feature/JIRA-456 in selected repos
gitm branch rename feature/JIRA-123 feature/JIRA-456

# Apply to all repos that have the branch
gitm branch rename feature/JIRA-123 feature/JIRA-456 --all

# Local rename only (skip remote)
gitm branch rename old-name new-name --no-remote
```

**Example output:**

```
Renaming "feature/JIRA-123" → "feature/JIRA-456" in 2 repository(ies)…

[auth-service        ] ✓ renamed feature/JIRA-123 → feature/JIRA-456 (local + remote)
[frontend            ] ✓ renamed feature/JIRA-123 → feature/JIRA-456 (local + remote)

Done: 2 succeeded
```

---

### `gitm status`

Show a summary of all registered repositories: current branch, dirty state, and commits ahead/behind origin. Runs in **parallel** (fetches remote info concurrently).

```
gitm status
```

**Example output:**

```
Fetching status for 4 repositories…

REPO                    BRANCH                    DIRTY         REMOTE
────────────────────────────────────────────────────────────────────────────────
api-gateway             main                      clean         up to date
auth-service            feature/JIRA-456          2 modified    3 behind
frontend                feature/JIRA-456          clean         up to date
payment-svc             main                      clean         5 ahead
```

**Column descriptions:**

| Column | Description |
|---|---|
| `REPO` | Repository name |
| `BRANCH` | Currently checked-out branch |
| `DIRTY` | `clean` if no uncommitted changes; otherwise shows the number of modified files |
| `REMOTE` | Commits ahead/behind `origin`. `up to date` if in sync. |

> **Note:** This command runs `git fetch --quiet` on each repository to get accurate ahead/behind counts. It may take a few seconds depending on your network.

---

### `gitm update`

Pull the latest changes on the **current branch** of every repository in parallel. Unlike `checkout master`, this does **not** switch branches.

```
gitm update
```

**Behaviour:**

1. For each registered repository (in parallel):
   - Checks for uncommitted changes — skips if dirty.
   - Runs `git pull --ff-only` on the current branch.
2. Streams results live with a summary.

**Use case:** You've been working on a feature branch for a while and want to pull in the latest changes your teammates pushed to the same branch across multiple repos.

**Example output:**

```
Pulling current branch for 4 repositories…

[api-gateway        ] ✓ on main — already up to date
[auth-service       ] ✓ on feature/JIRA-456 — pulled
[frontend           ] ⚠ SKIPPED: uncommitted changes — stash or commit first
[payment-svc        ] ✓ on main — already up to date

Done: 3 succeeded, 1 skipped
```

---

## How It Works

### Parallel Execution

Every multi-repo operation uses a concurrent worker pool (`golang.org/x/sync/errgroup`) with a default concurrency limit of **10 parallel git operations**. Results are streamed to the terminal as each operation completes, so you don't wait for a slow repository to see the others' results.

### Default Branch Detection

`gitm` auto-detects each repository's default branch using this fallback chain:

1. `git symbolic-ref refs/remotes/origin/HEAD` — reads what origin considers the default.
2. Checks if a local branch named `main` exists.
3. Checks if a local branch named `master` exists.
4. Falls back to the current `HEAD` branch.

This is stored in the SQLite database when the repo is added and used by `checkout master` and `branch create`.

### Skip, Never Force

`gitm` **never** force-resets or stashes your work. If a repository has uncommitted changes when a checkout or pull is attempted, it is **skipped** and reported in the summary. Your work is always safe.

---

## Data Storage

gitm stores repository configuration in a SQLite database at:

```
~/.gitm/gitm.db
```

The database is created automatically on first run. It contains a single table:

```sql
CREATE TABLE repositories (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    name           TEXT     NOT NULL UNIQUE,    -- directory name
    path           TEXT     NOT NULL UNIQUE,    -- absolute path
    default_branch TEXT     NOT NULL,           -- auto-detected: main or master
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

To back up or migrate your configuration:

```bash
# Backup
cp ~/.gitm/gitm.db ~/gitm-backup.db

# Move to a new machine: copy the binary and the .db file
scp ~/.gitm/gitm.db newmachine:~/.gitm/gitm.db
```

---

## Development

### Project Structure

```
cli-git-commands/
├── cmd/
│   └── gitm/
│       └── main.go              # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go              # Root cobra command
│   │   ├── repo.go              # repo add/list/remove
│   │   ├── checkout.go          # checkout master
│   │   ├── branch.go            # branch create/rename
│   │   ├── status.go            # status
│   │   └── update.go            # update
│   ├── config/
│   │   └── config.go            # App config & data dir
│   ├── db/
│   │   ├── db.go                # SQLite connection & migrations
│   │   └── repository.go        # Repository CRUD
│   ├── git/
│   │   └── git.go               # Git operations
│   ├── runner/
│   │   └── parallel.go          # Parallel execution engine
│   └── tui/
│       └── multiselect.go       # Bubbletea multi-select UI
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

### Build Targets

```bash
make build    # Compile to ./bin/gitm
make install  # Install to GOPATH/bin
make test     # Run all tests with race detector
make lint     # Run go vet + staticcheck
make clean    # Remove ./bin/
make tidy     # Tidy go.mod
make help     # Show all targets
```

### Running locally during development

```bash
# Build and run a command in one step
make run ARGS="repo list"
make run ARGS="checkout master"
make run ARGS="branch create feature/test --all"
```

### Adding a new command

1. Create `internal/cli/<command>.go`.
2. Define a function `func <command>Cmd() *cobra.Command`.
3. Register it in `internal/cli/root.go` by adding `root.AddCommand(<command>Cmd())`.

### Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.x | CLI framework |
| `modernc.org/sqlite` | v1.x | Pure-Go SQLite (no CGO) |
| `github.com/charmbracelet/bubbletea` | v1.x | TUI framework |
| `github.com/charmbracelet/bubbles` | v1.x | TUI components |
| `github.com/charmbracelet/lipgloss` | v1.x | Terminal styling |
| `github.com/fatih/color` | v1.x | Colored output |
| `golang.org/x/sync` | latest | `errgroup` for parallel ops |
