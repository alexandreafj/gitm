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
  - [repo rename](#gitm-repo-rename)
  - [checkout](#gitm-checkout)
  - [branch create](#gitm-branch-create)
  - [branch rename](#gitm-branch-rename)
  - [status](#gitm-status)
  - [update](#gitm-update)
  - [discard](#gitm-discard)
  - [commit](#gitm-commit)
  - [stash](#gitm-stash)
- [How It Works](#how-it-works)
- [Data Storage](#data-storage)
- [Development](#development)

---

## Why gitm?

When working across many repositories, daily git operations become repetitive:

| Without gitm | With gitm |
|---|---|
| `cd repo1 && git checkout main && git pull` × 23 repos | `gitm checkout master` |
| Checkout a feature branch in specific repos interactively | `gitm checkout` |
| Checkout a branch across all repos at once | `gitm checkout feature/JIRA-12345` |
| Manually `cd` into 6 repos to create a feature branch | `gitm branch create feature/JIRA-123` |
| Manually rename a branch in each repo + update remote | `gitm branch rename old-name new-name` |
| Forget which repos are dirty or behind origin | `gitm status` |
| Manually `cd` into each repo to stage + commit + push | `gitm commit` |
| Stash changes in specific repos before switching branches | `gitm stash` |
| Re-apply stashed work after switching back | `gitm stash pop` |

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

# Add with a custom alias (useful when two repos share the same directory name)
gitm repo add /path/to/www-api/v1 --alias www-api-v1
gitm repo add /path/to/docs-api/v1 --alias docs-api-v1

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

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--alias` | _(directory name)_ | Custom display name for the repository. Must be unique across all registered repos. Useful when two repos share the same directory name (e.g. two repos both named `v1`). |

**Examples:**

```bash
# Add a single repo
gitm repo add /home/user/work/api-gateway

# Add multiple repos at once
gitm repo add /home/user/work/api-gateway /home/user/work/auth-service /home/user/work/frontend

# Add the current directory
gitm repo add .

# Add two repos that share the same directory name using aliases
gitm repo add /home/user/work/www-api/v1 --alias www-api-v1
gitm repo add /home/user/work/docs-api/v1 --alias docs-api-v1
```

**Behaviour:**

- Validates that the path exists and is a git repository (`git rev-parse` check).
- Auto-detects the default branch (`main` or `master`) by inspecting `origin/HEAD`.
- The alias (display name) defaults to the directory base name. Use `--alias` to override — this is required when two repos share the same directory name.
- If the alias is already taken by another path, prints a clear error with a suggested `--alias` command.
- Stores the alias, path, and default branch in `~/.gitm/gitm.db`.

---

### `gitm repo list`

List all registered repositories.

```
gitm repo list
```

**Example output:**

```
#     ALIAS                     DEFAULT BRANCH  PATH
1     api-gateway               main            /home/user/work/api-gateway
2     auth-service              master          /home/user/work/auth-service
3     docs-api-v1               master          /home/user/work/docs-api/v1
4     frontend                  main            /home/user/work/frontend
5     www-api-v1                main            /home/user/work/www-api/v1

5 repository(ies) registered.
```

---

### `gitm repo remove`

Unregister a repository by alias. This only removes it from gitm's database — it does **not** delete the repository from disk.

```
gitm repo remove <alias>
gitm repo rm <alias>       # alias
```

**Arguments:**

| Argument | Description |
|---|---|
| `<alias>` | The repository alias as shown in `gitm repo list`. |

**Examples:**

```bash
gitm repo remove api-gateway
gitm repo rm www-api-v1
```

---

### `gitm repo rename`

Rename a registered repository's alias without removing and re-adding it. Useful for fixing ambiguous names on already-registered repos.

```
gitm repo rename <old-alias> <new-alias>
```

**Arguments:**

| Argument | Description |
|---|---|
| `<old-alias>` | The current alias as shown in `gitm repo list`. |
| `<new-alias>` | The new alias to use. Must not already be in use. |

**Examples:**

```bash
# Fix an ambiguous name after registration
gitm repo rename v1 www-api-v1
gitm repo rename v2 www-api-v2
```

---

### `gitm checkout`

Switch repositories to a branch and pull. Three modes of operation. Runs in **parallel**.

```
gitm checkout [branch]
```

**Modes:**

| Invocation | Behaviour |
|---|---|
| `gitm checkout` _(no args)_ | Interactive: multi-select repos, then type a branch name |
| `gitm checkout master` or `gitm checkout main` | Switch **all** repos to their configured default branch + pull |
| `gitm checkout <branch-name>` | Check out `<branch-name>` in **all** repos; skip with warning where it doesn't exist |

**Behaviour (all modes):**

- Repositories with uncommitted **tracked** changes are skipped (untracked files like `AGENTS.md` are safely ignored).
- Branch existence is checked locally first, then on the remote — skipped with a warning if neither has it.
- After checkout, runs `git pull --ff-only`.
- Streams results live with a final summary.

**Example — default branch:**

```
$ gitm checkout master

Checking out default branch and pulling for 4 repositories…

[api-gateway        ] ✓ on main — already up to date
[auth-service       ] ✓ on master — 3 files changed, 47 insertions(+)
[frontend           ] ⚠ SKIPPED: uncommitted changes (2 file(s)): M src/App.tsx, M package.json
[payment-svc        ] ✓ on main — already up to date

Done: 3 succeeded, 1 skipped
```

**Example — specific branch:**

```
$ gitm checkout feature/JIRA-12345

Checking out branch "feature/JIRA-12345" in 4 repositories…

[api-gateway        ] ✓ on feature/JIRA-12345 — already up to date
[auth-service       ] ✓ on feature/JIRA-12345 — pulled
[frontend           ] ⚠ SKIPPED: branch "feature/JIRA-12345" not found (local or remote)
[payment-svc        ] ⚠ SKIPPED: uncommitted changes (1 file(s))

Done: 2 succeeded, 2 skipped
```

**Example — interactive:**

```
$ gitm checkout

Select repositories to checkout
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

▶ [✓] api-gateway   /home/user/work/api-gateway
  [✓] auth-service   /home/user/work/auth-service
  [ ] frontend       /home/user/work/frontend

2/3 selected

Branch to checkout
Type the branch name  •  enter to confirm  •  esc to cancel

feature/JIRA-12345

Checking out branch "feature/JIRA-12345" in 2 repositories…
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

Show a summary of all registered repositories: current branch, dirty state, and commits ahead/behind origin. Runs in **parallel** with no network calls by default.

```
gitm status [flags]
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--fetch` | false | Run `git fetch` on all repos first for up-to-date remote numbers (slower, requires network). |

**Example output (fast mode, no network):**

```
Collecting status for 11 repositories…

REPO                    BRANCH                    DIRTY         REMOTE
────────────────────────────────────────────────────────────────────────────────
api-gateway             feature/PROJ-101          2 modified    up to date
auth-service            feature/PROJ-202          1 modified    up to date
billing                 master                    clean         14 behind
data-pipeline           feature/PROJ-101          1 modified    up to date
frontend                master                    1 modified    up to date
notifications           feature/PROJ-303          clean         up to date
payments                feature/PROJ-101          2 modified    up to date
reporting               master                    2 modified    up to date
search                  master                    12 modified   4 behind
user-service            feature/PROJ-303          1 modified    up to date
worker                  master                    1 modified    up to date
```

**Column descriptions:**

| Column | Description |
|---|---|
| `REPO` | Repository name |
| `BRANCH` | Currently checked-out branch |
| `DIRTY` | `clean` if no uncommitted changes; otherwise shows the number of modified files |
| `REMOTE` | Commits ahead/behind `origin`. Based on the last known remote state (no network call). Use `--fetch` for current numbers. |

**Examples:**

```bash
# Fast: instant, uses cached remote tracking info
gitm status

# Accurate: fetch from origin first, then show status (takes a few seconds)
gitm status --fetch
```

> **Performance note:** By default, `gitm status` is near-instant because it doesn't fetch from origin. The ahead/behind numbers reflect the last known state of remote branches. Use `--fetch` if you need up-to-the-second accuracy from the remote.

---

### `gitm discard`

Interactively select which repositories to discard uncommitted changes in. Only repositories that actually have changes are shown in the selection list — if none of your repos have uncommitted changes, the command exits immediately with a message.

```
gitm discard
```

> **WARNING:** This operation is irreversible. Discarded changes cannot be recovered.

**What it does per selected repository:**

```
git checkout -- .   → discard modifications to tracked files
git clean -fd       → remove untracked files and directories
```

**Behaviour:**

1. Scans all registered repositories for uncommitted changes.
2. If **none** are dirty, prints `Nothing to discard — all repositories are clean.` and exits.
3. If some are dirty, shows a summary of how many files each has modified, then opens the interactive multi-select showing **only the dirty repos**.
4. Executes discard in parallel on all selected repositories.
5. Streams results live.

**Example flow:**

```
3 repositories with uncommitted changes:

  repo             2 file(s) modified
  repo             2 file(s) modified
  repo             12 file(s) modified

WARNING: Select repositories to discard changes in (irreversible)
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [ ] repo          /home/user/work/repo
▶ [✓] repo  /home/user/work/repo
  [ ] repo  /home/user/work/repo

1/3 selected
```

After confirming:

```
Discarding changes in 1 repository(ies)…

[repo         ] ✓ discarded 2 file(s)

Done: 1 succeeded
```

**Example when nothing to discard:**

```
$ gitm discard
Nothing to discard — all repositories are clean.
```

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

### `gitm commit`

Interactively stage files and commit across dirty repositories. Walks you through each selected repository **sequentially** — pick files, write a message, and push.

```
gitm commit [flags]
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--no-push` | false | Commit but skip `git push` after each commit. |

**What it does:**

1. **Scans** all registered repositories for uncommitted changes.
2. **Filters** to dirty repos only — repos on their default branch are shown but marked `⛔ protected branch` and cannot be selected.
3. **Multi-select UI** — pick which repos you want to commit.
4. For each selected repo, **sequentially**:
   - **File picker** — shows all dirty files with colour-coded status prefixes (yellow `M`, green `A`, red `D`, dim `??`). Nothing is pre-selected.
   - **Commit message input** — single-line text input; rejects empty messages.
   - `git add -- <selected files>`
   - `git commit -m "<message>"`
   - `git push --set-upstream origin <branch>` (skipped with `--no-push`)
   - Live result printed per repo.
5. **Final summary** — `N committed, N skipped, N failed`.

**Example flow:**

```
Scanning repositories for uncommitted changes…

Select repositories to commit
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [ ] api-gateway       /home/user/work/api-gateway
▶ [ ] auth-service      /home/user/work/auth-service
  [ ] main-repo         /home/user/work/main-repo   ⛔ protected branch

0/2 selected
```

After selecting repos, for each one:

```
━━━ auth-service ━━━

Select files to stage for auth-service
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [✓] M  src/auth/login.go
  [ ] ?? scratch.txt

1/2 selected

Commit message for auth-service
Type your commit message  •  enter to confirm  •  esc to cancel

fix: correct token expiry check

  ✓ Staged 1 file(s)
  ✓ Committed: [feature/JIRA-456 3a4b5c6] fix: correct token expiry check
  ✓ Pushed
```

Final summary:

```
Summary
───────────────────────
  ✓  auth-service (committed + pushed)
  ~  api-gateway

1 committed  0 skipped  0 failed
```

**Protected branch behaviour:**

Repos that are currently on their configured default branch (e.g. `main` or `master`) are shown greyed out in the selection list with an `⛔ protected branch` label and **cannot be toggled**. This prevents accidental direct commits to the default branch.

**Examples:**

```bash
# Interactive commit + push
gitm commit

# Interactive commit, skip push
gitm commit --no-push
```

---

### `gitm stash`

Manage git stashes across selected repositories. All subcommands require you to select repos via the interactive multi-select TUI before anything happens.

```
gitm stash
gitm stash apply
gitm stash pop
gitm stash list
```

#### `gitm stash` _(push)_

Scans all repos for uncommitted changes (including untracked files), shows only dirty repos in the multi-select, then runs `git stash push --include-untracked` with an auto-generated message on each selected repo in parallel.

```
$ gitm stash

Scanning repositories for uncommitted changes…

Select repositories to stash
↑/↓ or j/k  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

▶ [✓] repo1          /home/user/work/repo1
  [ ] repo2          /home/user/work/repo2

1/2 selected

Stashing changes in 1 repository(ies)…

[repo1                 ] ✓ stashed (gitm stash on feature/JIRA-456)

Done: 1 succeeded
```

#### `gitm stash apply`

Scans all repos for stash entries, shows only repos with stashes in the multi-select, then runs `git stash apply` (keeps the stash) on each selected repo in parallel.

#### `gitm stash pop`

Same as `apply`, but runs `git stash pop` — applies and removes the stash entry.

#### `gitm stash list`

Prints a table of all repos that have stash entries, with the count and top stash message.

```
$ gitm stash list

REPO          STASHES  TOP STASH
────────────────────────────────────────────────────
repo1          1        On feature/JIRA-456: gitm stash on feature/JIRA-456
repo2          2        On master: gitm stash on master

2 repository(ies) with stash entries.
```

---

## How It Works

### Parallel Execution

Every multi-repo operation uses a concurrent worker pool (`golang.org/x/sync/errgroup`) with a default concurrency limit of **10 parallel git operations**. Results are streamed to the terminal as each operation completes, so you don't wait for a slow repository to see the others' results.

### Performance Optimizations

**`gitm status` is optimized for speed:**
- By default, it **does not fetch from origin**, making it nearly instant (~2 seconds for 11 repos) because it only reads local git state and uses cached remote tracking info.
- Use the `--fetch` flag if you need accurate up-to-the-second ahead/behind numbers from the remote (requires network calls).

**Why this matters:** When you're checking the status of 20+ repos multiple times a day, you want it to be fast. The cached remote state is accurate enough for most daily workflows — you only need `--fetch` when preparing to merge or push.

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
    name           TEXT     NOT NULL,               -- auto-detected directory name
    alias          TEXT     NOT NULL UNIQUE,        -- display name (user-controlled)
    path           TEXT     NOT NULL UNIQUE,        -- absolute path
    default_branch TEXT     NOT NULL,               -- auto-detected: main or master
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
│   │   ├── repo.go              # repo add/list/remove/rename
│   │   ├── checkout.go          # checkout master
│   │   ├── branch.go            # branch create/rename
│   │   ├── status.go            # status
│   │   ├── update.go            # update
│   │   ├── discard.go           # discard
│   │   ├── commit.go            # commit
│   │   └── stash.go             # stash / stash apply / stash pop / stash list
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
│       ├── multiselect.go       # Bubbletea multi-select UI (with disabled-item support)
│       ├── fileselect.go        # File picker UI (porcelain status, colour-coded)
│       └── textinput.go         # Single-line commit message input
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
