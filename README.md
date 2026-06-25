<div align="center">

# gitm

**Multi-Repository Git Manager**

Run git operations across dozens of repositories in parallel — checkout, pull, commit, stash, reset, track — from one command.

[![CI](https://github.com/alexandreafj/gitm/actions/workflows/ci.yml/badge.svg)](https://github.com/alexandreafj/gitm/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/alexandreafj/gitm?sort=semver&cacheSeconds=300)](https://github.com/alexandreafj/gitm/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alexandreafj/gitm)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexandreafj/gitm)](https://goreportcard.com/report/github.com/alexandreafj/gitm)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)
[![License](https://img.shields.io/github/license/alexandreafj/gitm)](LICENSE)

</div>

---

## Highlights

- **Parallel by default** — 10 concurrent git ops, live-streamed output.
- **Safe** — never force-resets your work; dirty repos are skipped, not clobbered.
- **Interactive TUI** — multi-select repos and files with bubbletea.
- **Self-updating (manual installs)** — `gitm upgrade` pulls signed binaries from GitHub Releases on macOS/Linux manual installs.
- **Zero config** — single SQLite file at `~/.gitm/gitm.db`, no daemons.

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
  - [group](#gitm-group)
  - [checkout](#gitm-checkout)
  - [branch create](#gitm-branch-create)
  - [branch rename](#gitm-branch-rename)
  - [branch delete](#gitm-branch-delete)
  - [status](#gitm-status)
  - [update](#gitm-update)
  - [sync](#gitm-sync)
  - [discard](#gitm-discard)
  - [commit](#gitm-commit)
  - [stash](#gitm-stash)
  - [reset](#gitm-reset)
  - [track](#gitm-track)
  - [untrack](#gitm-untrack)
  - [doctor](#gitm-doctor)
  - [upgrade](#gitm-upgrade)
- [How It Works](#how-it-works)
- [Data Storage](#data-storage)
- [Testing](#testing)
  - [Running Tests](#running-tests)
  - [Test Stats](#test-stats)
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
| Undo local commits and clean up history before pushing | `gitm reset` |
| Rewrite pushed history safely across multiple repos | `gitm reset --hard --commits 2` then approve force-push |

---

## Installation

### Homebrew Cask (macOS)

```bash
brew tap alexandreafj/gitm
brew install --cask gitm
```

### Scoop (Windows)

```powershell
scoop bucket add gitm https://github.com/alexandreafj/scoop-gitm
scoop install gitm
```

### Shell script (macOS / Linux)

Auto-detects your OS and architecture, downloads the binary, verifies the SHA-256 checksum, and installs to `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreafj/gitm/master/install.sh | sh
```

To install a specific version or to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreafj/gitm/master/install.sh | VERSION=v1.0.12 INSTALL_DIR="$HOME/.local/bin" sh
```

### Self-update

For manual macOS/Linux installs, gitm can update itself with signature verification:

```bash
gitm upgrade
```

If installed via a package manager, use the package manager upgrade flow instead:

```bash
brew upgrade --cask gitm   # Homebrew (macOS)
scoop update gitm          # Scoop (Windows)
```

### Download pre-built binary

Pre-built binaries for all major platforms are also available on the [GitHub Releases](https://github.com/alexandreafj/gitm/releases) page.

| Platform | Binary |
|---|---|
| macOS (Apple Silicon) | `gitm-macos-arm64` |
| macOS (Intel) | `gitm-macos-x86_64` |
| Linux (x86_64) | `gitm-linux-amd64` |
| Linux (ARM64) | `gitm-linux-arm64` |
| Windows (x86_64) | `gitm-windows-amd64.exe` |

```bash
# Example: macOS Apple Silicon
curl -L https://github.com/alexandreafj/gitm/releases/latest/download/gitm-macos-arm64 -o gitm
chmod +x gitm
sudo mv gitm /usr/local/bin/
```

#### Verify installation

```bash
gitm --help
```

### Prerequisites (building from source)

- [Go 1.26+](https://golang.org/dl/)
- `git` available in your `PATH`

### From source

```bash
# Clone the repository
git clone https://github.com/alexandreafj/gitm.git
cd gitm

# Build and install to GOPATH/bin
make install

# Verify installation
gitm --help
```

### Verification

On manual macOS/Linux installs, `gitm upgrade` verifies the signature on `checksums.txt` against this repo's release workflow before installing any new binary:

- The release workflow signs `checksums.txt` with [cosign](https://github.com/sigstore/cosign) in keyless mode (OIDC-bound to `release.yml` on a tagged push). The signature, certificate, and Rekor transparency-log proof are bundled into `checksums.txt.bundle` and uploaded with each release.
- `gitm upgrade` downloads the bundle, verifies it against Sigstore's public-good trust root, and aborts on any failure.
- Releases that predate signing (older than this feature) fall back to SHA-256 verification with a warning.

To verify a manually-downloaded binary outside of `gitm upgrade`:

```bash
# Download the binary, checksums, and bundle for your release of choice
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/gitm-macos-arm64
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/checksums.txt
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/checksums.txt.bundle

# Verify the signature on checksums.txt (requires cosign installed)
cosign verify-blob --bundle checksums.txt.bundle \
  --certificate-identity-regexp '^https://github\.com/alexandreafj/gitm/\.github/workflows/release\.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Then verify the binary against the (now trusted) checksums file
sha256sum -c checksums.txt --ignore-missing
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

# Got a folder full of repos? Register them all at once
gitm repo add /path/to/projects --auto-detect

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
gitm repo add <path> --group <name>
```

**Arguments:**

| Argument | Description |
|---|---|
| `<path>` | Absolute or relative path to a git repository. Use `.` for the current directory. |

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--alias` | _(directory name)_ | Custom display name for the repository. Must be unique across all registered repos. Useful when two repos share the same directory name (e.g. two repos both named `v1`). Cannot be combined with `--auto-detect`. |
| `--auto-detect` | false | Scan the immediate subdirectories of the given path and register every git repository found. Skips plain directories and hidden directories (names starting with `.`). Cannot be combined with `--alias`. |
| `--depth` | 1 | How many directory levels to scan when using `--auto-detect`. Use `--depth 2` when repos are nested inside grouping folders (e.g. `project/v1`). Requires `--auto-detect`. |
| `--group`, `-g` | _(none)_ | Also add the repository to an existing custom group. Can be repeated or comma-separated. The repo is always added to the built-in `all` group automatically. |

**Examples:**

```bash
# Add a single repo
gitm repo add /home/user/work/api-gateway

# Add a repo and place it in an existing group
gitm repo add /home/user/work/api-gateway --group backend

# Add multiple repos at once
gitm repo add /home/user/work/api-gateway /home/user/work/auth-service /home/user/work/frontend

# Add the current directory
gitm repo add .

# Add two repos that share the same directory name using aliases
gitm repo add /home/user/work/www-api/v1 --alias www-api-v1
gitm repo add /home/user/work/docs-api/v1 --alias docs-api-v1

# Scan a parent folder and register every git repo found inside it
gitm repo add /home/user/work --auto-detect

# Scan two levels deep to find repos in subfolders (e.g. api-group/v1, api-group/v2)
gitm repo add /home/user/work --auto-detect --depth 2
```

**`--auto-detect` example output:**

```
$ gitm repo add /home/user/work --auto-detect

Found 4 git repository(ies) in /home/user/work

  ✓ added api-gateway (default branch: main)
  ✓ added auth-service (default branch: master)
  ✓ added frontend (default branch: main)
  ✓ added payment-svc (default branch: main)

4 repository(ies) registered. Run `gitm repo list` to see all.
```

If some repos are already registered, they are reported as skipped (⚠) and do not cause an error:

```
$ gitm repo add /home/user/work --auto-detect

Found 4 git repository(ies) in /home/user/work

  ✓ added auth-service (default branch: master)
  ⚠ /home/user/work/api-gateway: already registered as "api-gateway"
  ⚠ /home/user/work/frontend: already registered as "frontend"
  ✓ added payment-svc (default branch: main)

2 repository(ies) registered. Run `gitm repo list` to see all.
```

**Behaviour:**

- Validates that the path exists and is a git repository (`git rev-parse` check).
- Auto-detects the default branch (`main` or `master`) by inspecting `origin/HEAD`.
- The alias (display name) defaults to the directory base name. Use `--alias` to override — this is required when two repos share the same directory name.
- If the alias is already taken by another path, prints a clear error with a suggested `--alias` command.
- Stores the alias, path, and default branch in `~/.gitm/gitm.db`.
- Every repository is automatically added to the built-in `all` group. Use `--group` only for extra custom memberships; the group must already exist.
- With `--auto-detect`: scans subdirectories up to `--depth` levels deep (default 1). When a git repo is found, its children are not scanned. Hidden directories (`.git`, `.cache`, etc.) are always skipped at every level.

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

### `gitm group`

Manage optional repository groups. Existing users do not need to do anything after upgrading: the database migration creates a built-in `all` group and backfills every registered repository into it automatically.

The `all` group is system-managed. It appears in `gitm group list` and `gitm group show all`, but it cannot be created, renamed, deleted, or edited manually.

```
gitm group list
gitm group show <name>
gitm group create <name>
gitm group rename <old-name> <new-name>
gitm group delete <name>
gitm group add <name> <repo-alias...>
gitm group remove <name> <repo-alias...>
```

**Subcommands:**

| Command | Description |
|---|---|
| `gitm group list` | List groups, including `all`, with repository counts. |
| `gitm group show <name>` | Show repositories in a group. |
| `gitm group create <name>` | Create a custom group. |
| `gitm group rename <old> <new>` | Rename a custom group. |
| `gitm group delete <name>` | Delete a custom group and its memberships. Repositories are not removed. |
| `gitm group add <name> <repo-alias...>` | Add registered repositories to a custom group. |
| `gitm group remove <name> <repo-alias...>` | Remove registered repositories from a custom group. |

**Behaviour:**

- Group names cannot be empty, contain spaces, or contain commas.
- Repository aliases must already exist before they can be added to a group.
- The `all` group always contains every registered repository and is updated automatically by `repo add` and `repo remove`.
- `--group all` on repo-aware commands is equivalent to no group filter.
- `--repo` and `--group` can be combined; gitm uses the intersection and preserves the explicit `--repo` order.

**Examples:**

```bash
# Create a group and add repos to it
gitm group create backend
gitm group add backend api-gateway auth-service

# Show groups and group contents
gitm group list
gitm group show backend
gitm group show all

# Rename or delete a custom group
gitm group rename backend services
gitm group delete services
```

**Example output:**

```
$ gitm group list

GROUP                     REPOS       TYPE
all                       5           built-in
backend                   2           custom
frontend                  1           custom
```

---

### `gitm checkout`

Switch repositories to a branch and pull. Three modes of operation. Runs in **parallel**.

```
gitm checkout [branch] [--repo alias1,alias2] [--group name]
```

**Modes:**

| Invocation | Behaviour |
|---|---|
| `gitm checkout` _(no args)_ | Interactive: multi-select repos, then type a branch name |
| `gitm checkout master` or `gitm checkout main` | Switch **all** repos to their configured default branch + pull |
| `gitm checkout <branch-name>` | Check out `<branch-name>` in **all** repos; skip with warning where it doesn't exist |

**Flags:**

| Flag | Shorthand | Description |
|---|---|---|
| `--repo` | `-r` | Limit checkout to specific repository aliases (comma-separated) |
| `--group` | `-g` | Limit checkout to repositories in a group. Combines with `--repo` as an intersection. |

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

**Example — specific repos only:**

```
$ gitm checkout master --repo=api-gateway,auth-service

Checking out default branch and pulling for 2 repositories…

[api-gateway        ] ✓ on main — already up to date
[auth-service       ] ✓ on master — 3 files changed, 47 insertions(+)

Done: 2 succeeded, 0 skipped
```

**Example — group only:**

```
$ gitm checkout master --group backend

Checking out default branch and pulling for 2 repositories…
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

Create a new branch in selected repositories. An interactive multi-select UI lets you choose which repositories to apply the operation to. Use `--repo` to skip the UI entirely and target specific repositories by alias.

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
| `--repo` | `-r` | _(none)_ | Comma-separated list of repository aliases to target. Bypasses the interactive selection UI. Takes precedence over `--all`. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

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

# Create in every repo from a group
gitm branch create feature/JIRA-456 --all --group backend

# Create only in specific repos by alias (no prompt)
gitm branch create feature/AA-1 --repo api-gateway,auth-service --group backend

# Create from a specific base branch
gitm branch create hotfix/critical-bug --from develop

# Target specific repos and use a custom base branch
gitm branch create feature/AA-1 --repo api-gateway --from develop
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
| `--repo` | `-r` | _(none)_ | Comma-separated list of repository aliases to target. Bypasses the interactive selection UI. Takes precedence over `--all`. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

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

# Rename only in specific repos by alias (no prompt)
gitm branch rename feature/JIRA-123 feature/JIRA-456 --repo api-gateway,auth-service --group backend

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

### `gitm branch delete`

Delete a branch across selected repositories — both locally and on the remote in one step. Runs in **parallel**.

```
gitm branch delete <branch-name> [flags]
```

**Arguments:**

| Argument | Description |
|---|---|
| `<branch-name>` | The branch to delete. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--all` | `-a` | false | Apply to all repositories that have the branch. |
| `--force` | `-f` | false | Force-delete branches with unmerged commits (`git branch -D` instead of `-d`). |
| `--no-remote` | — | false | Only delete the local branch. Skip deleting the branch on origin. |
| `--repo` | `-r` | _(none)_ | Comma-separated list of repository aliases to target. Bypasses the interactive selection UI. Takes precedence over `--all`. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

**Behaviour:**

1. Filters the registered repositories to only those that have the branch (locally, or on origin unless `--no-remote` is set).
2. Selects repositories:
   - Interactive: opens the multi-select UI showing only the matching repositories.
   - `--all` / `--repo`: skips the UI and asks for a single `y/N` confirmation listing the target repositories.
3. For each selected repo (in parallel):
   - `git branch -d <branch-name>` — deletes locally (`-D` when `--force`).
   - `git push origin --delete <branch-name>` — deletes the remote branch if it exists (skipped with `--no-remote`).
4. Streams results live.

**Safety:**

- The local delete uses `git branch -d`, which refuses branches with unmerged commits. Pass `--force` to delete them anyway.
- The repository's default branch (`main`/`master`) is never deleted — it is skipped.
- A branch that is currently checked out is skipped — switch away from it first.

**Examples:**

```bash
# Interactive: delete feature/JIRA-123 in selected repos
gitm branch delete feature/JIRA-123

# Apply to all repos that have the branch
gitm branch delete feature/JIRA-123 --all

# Delete only in specific repos by alias (asks for confirmation)
gitm branch delete feature/JIRA-123 --repo api-gateway,auth-service --group backend

# Force-delete a branch with unmerged commits
gitm branch delete feature/JIRA-123 --force

# Delete only the local branch, keep it on origin
gitm branch delete feature/JIRA-123 --no-remote
```

**Example output:**

```
Branch "feature/JIRA-123" will be deleted in 2 repository(ies):
  - auth-service
  - frontend
Delete branch "feature/JIRA-123"? [y/N] y

Deleting "feature/JIRA-123" in 2 repository(ies)…

[auth-service        ] ✓ deleted feature/JIRA-123 (local + remote)
[frontend            ] ✓ deleted feature/JIRA-123 (local + remote)

Done: 2 succeeded
```

---

### `gitm status`

Show a summary of all registered repositories: current branch, dirty state, and commits ahead/behind origin. Runs in **parallel** with no network calls by default.

```
gitm status [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--fetch` | — | false | Run `git fetch` on all repos first for up-to-date remote numbers (slower, requires network). |
| `--repo` | `-r` | _(all repos)_ | Limit output to specific repository aliases (comma-separated). |
| `--group` | `-g` | _(all repos)_ | Limit output to repositories in a group. Combines with `--repo` as an intersection. |

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

# Show status for specific repos only
gitm status -r api-gateway
gitm status -r api-gateway,auth-service --group backend --fetch

# Show status for a group
gitm status --group backend
```

> **Performance note:** By default, `gitm status` is near-instant because it doesn't fetch from origin. The ahead/behind numbers reflect the last known state of remote branches. Use `--fetch` if you need up-to-the-second accuracy from the remote.

---

### `gitm discard`

Interactively select which repositories and **which files** to discard uncommitted changes in. Only repositories that actually have changes are shown in the selection list — if none of your repos have uncommitted changes, the command exits immediately with a message.

```
gitm discard [flags]
```

> **WARNING:** This operation is irreversible. Discarded changes cannot be recovered.

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated), bypasses interactive repo selection. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

**What it does per selected file (based on status):**

| File status | Git command(s) | Effect |
|---|---|---|
| Modified tracked (` M`, `M `, `MM`) | `git reset HEAD -- <file>` + `git checkout -- <file>` | Reverts to last committed version |
| Staged new file (`A `) | `git reset HEAD -- <file>` + `git clean -fd -- <file>` | Unstages and removes the file |
| Untracked file/dir (`??`) | `git clean -fd -- <file>` | Removes the file or directory |

**Behaviour:**

1. Scans all registered repositories (or those specified with `--repo` / `--group`) for uncommitted changes.
2. If **none** are dirty, prints `Nothing to discard — all repositories are clean.` and exits.
3. If some are dirty, shows a summary of how many files each has modified, then opens the interactive multi-select showing **only the dirty repos** (skipped when `--repo` is used).
4. For each selected repo, opens a **file picker** where you choose exactly which files to discard. **No files are pre-selected** — you must explicitly pick every file you want gone.
5. Discards only the selected files in each repo.
6. Prints a per-repo summary.

**Example flow:**

```
3 repositories with uncommitted changes:

  api-gateway            2 file(s) modified
  auth-service           5 file(s) modified
  frontend               12 file(s) modified

WARNING: Select repositories to discard changes in (irreversible)
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [ ] api-gateway        /home/user/work/api-gateway
▶ [✓] auth-service       /home/user/work/auth-service
  [ ] frontend           /home/user/work/frontend

1/3 selected
```

After selecting a repo, the **file picker** appears:

```
━━━ auth-service ━━━
Select files to discard in auth-service (irreversible)
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

▶ [ ] M  src/handler.go
  [✓] M  src/config.go
  [ ] ?? tmp/debug.log
  [✓] A  src/new_service.go

2/4 selected
```

After confirming:

```
  ✓ Discarded 2 file(s):
       src/config.go
       src/new_service.go

Summary
───────────────────────
  ✓  auth-service (2 file(s) discarded)

1 discarded  0 skipped  0 failed
```

**Example with --repo flag:**

```
$ gitm discard --repo api-gateway
$ gitm discard -r api-gateway,auth-service --group backend
$ gitm discard --group backend
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
gitm update [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit update to specific repository aliases (comma-separated). |
| `--group` | `-g` | _(all repos)_ | Limit update to repositories in a group. Combines with `--repo` as an intersection. |

**Behaviour:**

1. If `--repo` or `--group` is specified, only matching repos are updated. Otherwise, all registered repos are updated.
2. For each repository (in parallel):
   - Checks for uncommitted changes — skips if dirty.
   - Runs `git pull --ff-only` on the current branch.
   - If the remote branch no longer exists (e.g. deleted after a PR merge), automatically switches to the default branch and pulls that instead.
3. Streams results live with a summary.
4. If a `--repo` alias doesn't match any registered repository, the command exits with an error before pulling anything.

**Use case:** You've been working on a feature branch for a while and want to pull in the latest changes your teammates pushed to the same branch across multiple repos.

**Examples:**

```bash
# Update all registered repos
gitm update

# Update a single repo by alias
gitm update --repo=api-gateway

# Update multiple specific repos
gitm update --repo=api-gateway,auth-service

# Update repos in a group
gitm update --group backend

# Short form
gitm update -r api-gateway,auth-service -g backend
```

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

### `gitm sync`

Merge a branch into the branch each repository is **currently on** — in parallel. By default that branch is each repo's **default branch** (`main`/`master`, auto-detected per repo); pass an optional `[branch]` argument to merge a different branch instead. This replaces the manual, per-repo routine of pulling the latest `master`/`main` and merging it into your working branch with `git merge master` by hand.

```
gitm sync [branch] [flags]
```

**Arguments:**

| Argument | Description |
|---|---|
| `[branch]` | _(optional)_ Branch to merge into each repo's current branch. Omit it to use each repo's default branch (`main`/`master`). Repos where the branch does not exist locally or on `origin` are skipped. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(prompt)_ | Sync only the listed repository aliases (comma-separated). Skips the interactive picker. |
| `--all` | `-a` | `false` | Sync every registered repository without prompting. |
| `--group` | `-g` | _(all repos)_ | Limit sync candidates to repositories in a group. Combines with `--repo` as an intersection. |

**Selection modes:**

| Invocation | Behaviour |
|---|---|
| `gitm sync` | Interactive — pick repositories via the TUI. |
| `gitm sync <branch>` | Merge `<branch>` (instead of the default branch) into each repo's current branch. |
| `gitm sync --repo a,b` | Sync only repos `a` and `b` (no prompt). |
| `gitm sync <branch> --repo a,b` | Merge `<branch>` into repos `a` and `b` (no prompt). |
| `gitm sync --all` | Sync every registered repository (no prompt). |
| `gitm sync --group backend` | Interactive — pick from repositories in `backend`. |

**Behaviour (per repository, in parallel):**

1. Determines the target branch: the repository's default branch (`main` or `master`, from the value stored at `repo add`) unless a `[branch]` argument is given, in which case that branch is used for every repo.
2. **Skips** repos with uncommitted tracked changes (stash or commit first). Untracked files do not block the sync.
3. **Skips** repos already on the target branch (use `gitm update` to pull instead).
4. Fetches the latest target branch from `origin`, then merges `origin/<branch>` into the current branch (falls back to the local branch when there is no remote). Repos where the branch is missing both locally and on `origin` are **skipped**.
5. **Merge conflicts are left in place** — the repo is reported and kept in its merging state so you can resolve the conflicts and commit. A conflict is not treated as a failure; the command still exits 0.
6. Streams results live with a summary, followed by a list of any repos left with conflicts.

**Use case:** Your feature branch has drifted behind `master`/`main` (or a long-lived integration branch like `master-raw`) across several repos and you want to merge the latest changes into each one in a single step.

**Examples:**

```bash
# Interactively pick repos to sync with their default branch
gitm sync

# Sync every repo with its default branch
gitm sync --all

# Sync every repo in a group
gitm sync --all --group backend

# Merge a specific branch instead of the default branch
gitm sync master-raw

# Merge a specific branch into specific repos (optionally scoped to a group)
gitm sync master-raw --repo=api-gateway,auth-service --group backend

# Sync specific repos by alias
gitm sync --repo=api-gateway,auth-service --group backend

# Short form
gitm sync -r api-gateway
```

**Example output:**

```
Merging default branch into the current branch of 3 repository(ies)…

[api-gateway        ] ✓ merged main into feature/JIRA-456 — fast-forward
[auth-service       ] ⚠ SKIPPED: currently on "main" — nothing to merge (use `gitm update` to pull)
[frontend           ] ⚠ SKIPPED: merge conflict — 2 file(s) to resolve manually

Done: 1 succeeded, 2 skipped

1 repository(ies) have merge conflicts left for you to resolve:
  - frontend (/Users/me/code/frontend)
      conflict: src/app.tsx
      conflict: package.json

Resolve the conflicts in each repo, then `git add` + `git commit` (or `git merge --abort`).
```

---

### `gitm commit`

Interactively stage files and commit across dirty repositories. Walks you through each selected repository **sequentially** — pick files, write a message, and push. Use `--repo` to skip the selection UI and target specific repositories by alias.

```
gitm commit [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--no-push` | — | false | Commit but skip `git push` after each commit. |
| `--repo` | `-r` | _(none)_ | Comma-separated list of repository aliases to target. Bypasses the interactive multi-select UI. Non-dirty repos in the list are silently skipped. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

**What it does:**

1. **Scans** the registered repositories (all, or only those in `--repo` / `--group`) for uncommitted changes.
2. **Filters** to dirty repos only — repos on their default branch are shown but marked `⛔ protected branch` and cannot be selected.
3. **Multi-select UI** — pick which repos you want to commit. _(Skipped when `--repo` is provided — all dirty, unprotected matches proceed automatically.)_
4. For each selected repo, **sequentially**:
   - **File picker** — shows all dirty files with colour-coded status prefixes (yellow `M`, green `A`, red `D`, orange `U` for conflicts, dim `??`). Nothing is pre-selected.
   - **Commit message input** — single-line text input; rejects empty messages.
   - `git add -- <selected files>`
   - `git commit -m "<message>"` (during a merge, commits the entire index to complete the merge)
   - `git push --set-upstream origin <branch>` (skipped with `--no-push`)
   - Live result printed per repo.
5. **Final summary** — `N committed, N skipped, N failed`.

**Merge conflict resolution:**

If a repository is in the middle of a merge (e.g. after `git merge` produced conflicts), `gitm commit` handles it automatically. Resolve the conflicted files, select them in the file picker, and commit — gitm detects the merge state and uses a full-index commit (no pathspec) as git requires.

**Rebase / cherry-pick / revert guard:**

Repos with an in-progress rebase, cherry-pick, or revert are **skipped** with a message hinting to use `git <operation> --continue` instead.

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

# Commit only specific repos by alias (no selection prompt)
gitm commit --repo api-gateway,auth-service --group backend

# Commit specific repos and skip push
gitm commit --repo api-gateway --group backend --no-push
```

---

### `gitm stash`

Manage git stashes across selected repositories. By default, an interactive multi-select TUI lets you choose which repos to operate on. Use `--repo` / `-r` to bypass the UI and target specific repos by alias.

```
gitm stash [flags]
gitm stash apply [flags]
gitm stash pop [flags]
gitm stash list [flags]
```

**Flags (all subcommands):**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated), bypasses interactive selection. |
| `--group` | `-g` | _(all repos)_ | Limit to repositories in a group. Combines with `--repo` as an intersection. |

#### `gitm stash` _(push)_

Scans repos for uncommitted changes (including untracked files), shows only dirty repos in the multi-select, then runs `git stash push --include-untracked` with an auto-generated message on each selected repo in parallel.

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

Scans repos for stash entries, shows only repos with stashes in the multi-select, then runs `git stash apply` (keeps the stash) on each selected repo in parallel.

#### `gitm stash pop`

Same as `apply`, but runs `git stash pop` — applies and removes the stash entry.

#### `gitm stash list`

Prints a table of repos that have stash entries, with the count and top stash message.

```
$ gitm stash list

REPO          STASHES  TOP STASH
────────────────────────────────────────────────────
repo1          1        On feature/JIRA-456: gitm stash on feature/JIRA-456
repo2          2        On master: gitm stash on master

2 repository(ies) with stash entries.
```

**Examples:**

```bash
# Interactive — select repos via TUI
gitm stash

# Stash dirty repos from a group
gitm stash --group backend

# Stash specific repos by alias (no prompt)
gitm stash -r api-gateway,auth-service --group backend

# Apply stash to a specific repo
gitm stash apply -r api-gateway --group backend

# Pop stash from specific repos
gitm stash pop --repo=api-gateway,auth-service

# List stash entries for a specific repo
gitm stash list -r api-gateway --group backend
```

---

### `gitm reset`

Undo the last N commits across selected repositories in three safe modes: soft, mixed (default), or hard. Perfect for undoing local commits before pushing.

```
gitm reset [flags]
```

**Modes:**

| Mode | Effect | Use case |
|---|---|---|
| `--soft` | Moves HEAD back; **keeps changes staged** and ready to re-commit | Squash, amend, or reorganize commits before pushing |
| _(default, mixed)_ | Moves HEAD back; **unstages changes but keeps them in working tree** | Undo commits and re-stage selectively |
| `--hard` | Moves HEAD back AND **discards all changes** irreversibly | Completely discard unwanted commits and changes |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--commits` | — | 1 | Number of commits to undo (reset back N commits) |
| `--soft` | — | false | Keep changes staged after reset |
| `--hard` | — | false | Discard all changes (irreversible) |
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated), bypasses interactive selection. |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

> **⚠️ WARNING:** `--hard` is irreversible. Discarded changes cannot be recovered. Only use this when you're certain.

**Pre-flight Check:**

Before applying the reset:
1. Shows a summary table of:
   - Each repository and the commits that will be undone (by hash + message)
   - Which commits are already pushed to origin (⚠️ red flag)
2. Opens the interactive multi-select UI to choose which repos to reset
3. If any commits are already pushed, you'll be prompted once: **"Force-push to clean remote history? [y/N]"**
   - `--force-with-lease` is used (the safest form) to rewrite history safely
   - Only offered if you own those branches (careful: shared branches will break teammates' clones!)

**Behaviour:**

1. For each selected repo (in parallel):
   - Moves HEAD back N commits
   - Applies the chosen reset mode (soft/mixed/hard)
   - Reports which commits were undone

2. If any undone commits were already pushed:
   - You're warned with a red caution box
   - Prompted once for all repos: approve or skip the force-push
   - If approved: `git push --force-with-lease` is used to rewrite remote history

**Examples:**

```bash
# Undo last commit, keep changes staged (safest)
gitm reset --soft

# Undo last commit, unstage changes, keep in working tree (default)
gitm reset

# Undo last 3 commits, unstage changes
gitm reset --commits 3

# Undo last 2 commits, discard all changes (IRREVERSIBLE)
gitm reset --hard --commits 2

# Reset specific repos by alias (no selection prompt)
gitm reset -r api-gateway
gitm reset --soft -r api-gateway,auth-service --group backend

# Reset only repos in a group
gitm reset --group backend
```

**Example flow:**

```
$ gitm reset --commits 2

Mode:  mixed  —  HEAD moves back; changes are unstaged but kept in working tree
Scope:  last 2 commit(s) per repository

  api-gateway          [2 commit(s) already pushed — remote will need force-push]
    ↩ a1b2c3d feat: add authentication
    ↩ e4f5g6h fix: correct token validation

Select repositories to reset
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [✓] api-gateway        /home/user/work/api-gateway

1/1 selected

Applying mixed reset (HEAD~2) to 1 repository(ies)…

[api-gateway        ] ✓ mixed reset — undid 2 commit(s):
       ↩ a1b2c3d feat: add authentication
       ↩ e4f5g6h fix: correct token validation
       changes are unstaged but present in the working tree

Done: 1 succeeded

┌──────────────────────────────────────────────────────┐
│  CAUTION: Remote history rewrite                     │
│                                                      │
│  1 of the reset repo(s) had already-pushed commits  │
│  undone. Force-pushing will rewrite the remote      │
│  branch history. Anyone who has already pulled      │
│  those commits will need to hard-reset their local  │
│  branch. Only do this on branches that you own and  │
│  no one else is using.                              │
└──────────────────────────────────────────────────────┘

The following 1 repo(s) will be force-pushed:
  api-gateway        /home/user/work/api-gateway

Force-push to clean remote history? [y/N] y

Force-pushing 1 repository(ies)…

[api-gateway        ] ✓ force-pushed branch master to origin

Done: 1 succeeded
```

---

### `gitm track`

Start tracking untracked files across multiple repositories. Only repositories with untracked files are shown in the selection list.

```
gitm track [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated). |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |

**What it does:**

1. Scans all registered repositories for untracked files.
2. If **none** have untracked files, prints a message and exits.
3. Shows a summary of how many untracked files each repo has.
4. Opens the interactive multi-select to choose which repos to track files in.
5. For each selected repo, opens the file picker showing only untracked files.
6. Runs `git add` on the selected files.

**Example flow:**

```
$ gitm track

2 repositories with untracked files:

  api-gateway            3 untracked file(s)
  auth-service           1 untracked file(s)

Select repositories to track files in
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

▶ [✓] api-gateway       /home/user/work/api-gateway
  [ ] auth-service       /home/user/work/auth-service

1/2 selected

Select files to track for api-gateway
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [✓] ?? src/new-handler.go
  [✓] ?? src/new-handler_test.go
  [ ] ?? scratch.txt

2/3 selected

[api-gateway        ] ✓ tracked 2 file(s)

Done: 1 succeeded
```

**Examples:**

```bash
# Interactive — select repos and files
gitm track

# Track files only in specific repos by alias
gitm track --repo api-gateway,auth-service --group backend

# Track files in repos from a group
gitm track --group backend
```

---

### `gitm untrack`

Stop tracking files across multiple repositories. Files are removed from the git index but **remain on disk**. This is useful for accidentally committed files like `.env`, logs, or build artifacts.

```
gitm untrack [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated). |
| `--group` | `-g` | _(all repos)_ | Limit candidates to repositories in a group. Combines with `--repo` as an intersection. |
| `--path` | `-p` | _(all files)_ | Filter files by glob pattern or path prefix (e.g. `"*.env"`, `"public/"`). |

**What it does:**

1. Opens the interactive multi-select to choose which repos to untrack files from.
2. For each selected repo, shows tracked files in the file picker (filtered by `--path` if provided).
3. Runs `git rm --cached` on the selected files — removes from git's index only.
4. The files remain on disk untouched.

> **Tip:** After untracking a file, add it to `.gitignore` to prevent it from being tracked again.

**Example flow:**

```
$ gitm untrack

Select repositories to untrack files from
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

▶ [✓] api-gateway       /home/user/work/api-gateway
  [ ] auth-service       /home/user/work/auth-service

1/1 selected

Select files to untrack for api-gateway (files stay on disk)
↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel

  [ ] T  go.mod
  [ ] T  go.sum
  [✓] T  .env
  [✓] T  debug.log

2/15 selected

[api-gateway        ] ✓ untracked 2 file(s)

Done: 1 succeeded
```

**Examples:**

```bash
# Interactive — select repos and files
gitm untrack

# Untrack files only in specific repos by alias
gitm untrack --repo api-gateway --group backend

# Filter to only show .env files
gitm untrack --path "*.env"

# Filter to only show files under public/
gitm untrack --path "public/"

# Combine repo and path filters
gitm untrack --repo api-gateway --group backend --path "*.log"
```

---

### `gitm doctor`

Run read-only diagnostics across registered repositories and report common health issues before they interrupt a workflow.

```
gitm doctor [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--repo` | `-r` | _(all repos)_ | Limit to specific repository aliases (comma-separated). |
| `--group` | `-g` | _(all repos)_ | Limit diagnostics to repositories in a group. Combines with `--repo` as an intersection. |

**What it checks:**

1. The registered path still exists and is a directory.
2. The path is still the root of a git repository.
3. The current branch can be read and is not detached.
4. The configured default branch exists locally.
5. An `origin` remote is configured.
6. The current branch has an upstream.
7. The working tree has uncommitted changes.
8. A merge, rebase, cherry-pick, revert, or bisect operation is in progress.

**Behaviour notes:**

- `OK` means the repository passed every diagnostic.
- `WARN` means the repository is usable but may need attention, such as a dirty working tree or missing upstream.
- `ERROR` means the registered repository is broken or cannot be inspected, such as a missing path or non-git directory.
- The command exits non-zero only when one or more repositories have `ERROR` status.
- The command does not fetch, pull, push, checkout, modify files, or update the database.

**Example output:**

```
$ gitm doctor

Checking 3 registered repository(ies)…

REPO                    STATUS   DETAILS
──────────────────────────────────────────────────────────────────────────────────────────
api-gateway             OK       healthy
auth-service            WARN     working tree has uncommitted changes; current branch has no upstream
old-worker              ERROR    path is not accessible: stat /home/user/work/old-worker: no such file or directory
```

**Examples:**

```bash
# Check every registered repository
gitm doctor

# Check only specific repos by alias
gitm doctor --repo api-gateway,auth-service --group backend

# Check a group
gitm doctor --group backend
```

---

### `gitm upgrade`

Self-update gitm to the latest release from GitHub for manual macOS/Linux installs. Downloads the correct binary for your platform, verifies the checksum, and replaces the current binary — no manual download needed.

```
gitm upgrade
```

If you installed `gitm` with a package manager, use:

```bash
brew upgrade --cask gitm   # Homebrew (macOS)
scoop update gitm          # Scoop (Windows)
```

**What it does:**

1. Queries the [GitHub Releases API](https://github.com/alexandreafj/gitm/releases) for the latest version.
2. Compares against the currently installed version (`gitm --version`).
3. Detects your OS and architecture to download the correct binary.
4. Downloads the binary and `checksums.txt`, then verifies SHA256 integrity.
5. Atomically replaces the current binary (backs up the old one, swaps in the new one).
6. Sets executable permissions on Linux/macOS (`chmod 755`).

**Supported platforms:**

| Platform | Binary |
|---|---|
| macOS (Apple Silicon) | `gitm-macos-arm64` |
| macOS (Intel) | `gitm-macos-x86_64` |
| Linux (x86_64) | `gitm-linux-amd64` |
| Linux (ARM64) | `gitm-linux-arm64` |
| Windows (x86_64) | `gitm-windows-amd64.exe` |

**Example — upgrade available:**

```
$ gitm upgrade

Checking for updates... found v1.1.0
Downloading gitm-macos-arm64... done
Verifying checksum... ok
Updated gitm: v1.0.6 → v1.1.0
```

**Example — already up to date:**

```
$ gitm upgrade

Checking for updates... already up to date (v1.1.0)
```

**Version check:**

```bash
# See your current version
gitm --version
```

> **Note:** This command does not require database access — it works even if `~/.gitm/gitm.db` doesn't exist yet.
>
> **Note:** `gitm upgrade` is disabled for package-managed installs (Homebrew/Scoop) and disabled on Windows.

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

The database is created automatically on first run. When gitm opens the database after an upgrade, it automatically applies missing migrations; users do not run migration commands manually. Group support creates the built-in `all` group and backfills every existing repository into it.

It contains these tables:

```sql
CREATE TABLE repositories (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    name           TEXT     NOT NULL,               -- auto-detected directory name
    alias          TEXT     NOT NULL UNIQUE,        -- display name (user-controlled)
    path           TEXT     NOT NULL UNIQUE,        -- absolute path
    default_branch TEXT     NOT NULL,               -- auto-detected: main or master
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE groups (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL UNIQUE,             -- includes built-in "all"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_repositories (
    group_id      INTEGER NOT NULL,
    repository_id INTEGER NOT NULL,
    PRIMARY KEY (group_id, repository_id),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
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

## Testing

### Running Tests

```bash
# Run all tests with race detection
make test

# Run tests verbosely
go test ./... -v -race -timeout 60s

# Run a specific package's tests
go test ./internal/cli/... -v -race

# Run a single test by name
go test ./internal/cli/... -v -race -run TestResetSoft
```

### Test Stats

| Metric | Count |
|---|---|
| Test files | 46 |
| Test functions | 424 |
| Language | Go |

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
│   │   ├── group.go             # group list/show/create/rename/delete/add/remove
│   │   ├── checkout.go          # checkout master
│   │   ├── branch.go            # branch create/rename/delete
│   │   ├── status.go            # status
│   │   ├── update.go            # update
│   │   ├── sync.go              # sync (merge default branch into current branch)
│   │   ├── discard.go           # discard
│   │   ├── commit.go            # commit
│   │   ├── stash.go             # stash / stash apply / stash pop / stash list
│   │   ├── reset.go             # reset --soft / --hard with force-push support
│   │   ├── track.go             # start tracking untracked files
│   │   ├── untrack.go           # stop tracking files (git rm --cached)
│   │   ├── doctor.go            # repository health diagnostics
│   │   └── upgrade.go           # self-update from GitHub releases
│   ├── config/
│   │   └── config.go            # App config & data dir
│   ├── db/
│   │   ├── db.go                # SQLite connection & migrations
│   │   ├── group.go             # Repository group CRUD and memberships
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

### Adding a new command

1. Create a feature branch: `git checkout -b feat/<command-name>`.
2. Create `internal/cli/<command>.go`.
3. Define a function `func <command>Cmd() *cobra.Command`.
4. Register it in `internal/cli/root.go` by adding `root.AddCommand(<command>Cmd())`.
5. If the command doesn't need DB access, add its name to the skip list in `PersistentPreRunE`.
6. Create `internal/cli/<command>_test.go` with unit tests (real git repos, no mocks).
7. Update this `README.md`: add to Table of Contents, Commands Reference, and Project Structure.
8. Run `make test && make lint` before committing.

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

---

## Contributing

See [`AGENTS.md`](./AGENTS.md) for the development workflow, coding standards, and the (short) list of approved dependencies. Briefly: feature branch off `master`, real git in tests (no mocks), `make lint && make test` before pushing.

## License

[MIT](./LICENSE) — see the LICENSE file for details.
