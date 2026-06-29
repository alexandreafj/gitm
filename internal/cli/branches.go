package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func branchesCmd() *cobra.Command {
	var (
		fetchRemote bool
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "branches [target-branch]",
		Short: "Show a branch dashboard across all registered repositories",
		Long: `Display a per-repository branch dashboard.

Without an argument, each row describes the branch the repository is currently
on: its upstream tracking branch, how many commits it is ahead/behind that
upstream, and whether it has been merged into the repository's default branch.

With a target branch argument (e.g. gitm branches feature/JIRA-123), each row
focuses on that branch across every repository: whether it exists locally and/or
on origin, which branch the repo is currently on (a ● marks repos checked out on
the target), and the target's upstream, ahead/behind, and merged-into-default
state. Repositories that do not have the target show — for the remaining columns.

Like gitm status, this command is offline by default: remote branch existence and
ahead/behind numbers come from the last-fetched origin refs. Pass --fetch to
refresh from origin first for up-to-date numbers (slower, requires network).

Use --repo / -r to limit output to specific repositories by alias.
Use --group / -g to limit output to repositories in a group.
When both are provided, gitm shows only aliases that also belong to the group.`,
		Example: `  gitm branches
  gitm branches feature/JIRA-123
  gitm branches feature/JIRA-123 --fetch
  gitm branches feature/JIRA-123 -r api-gateway,auth-service
  gitm branches -g backend`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = strings.TrimSpace(args[0])
			}
			return runBranches(target, fetchRemote, repoAliases, groupName)
		},
	}

	cmd.Flags().BoolVar(&fetchRemote, "fetch", false, "Fetch from origin before reading remote branch state (slower but accurate)")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &groupName)
	return cmd
}

type mergedState int

const (
	mergedUnknown mergedState = iota // not computable — shown as "—"
	mergedYes
	mergedNo
	mergedDefault // the subject branch is the default branch
)

type branchInfo struct {
	name        string // repo alias
	current     string // branch the repo is currently on
	targetState string // target mode only: local+remote / local only / remote only / missing
	onTarget    bool   // repo is currently checked out on the target branch
	hasSubject  bool   // the subject branch is available locally for ahead/behind/upstream
	upstream    string // upstream tracking ref of the subject branch, "" if none
	ahead       int
	behind      int
	merged      mergedState
	err         string
}

func runBranches(target string, fetchRemote bool, repoAliases []string, groupName string) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}

	printBranchesHeader(target, fetchRemote, len(repos))

	infos := make([]branchInfo, len(repos))
	done := make(chan struct{}, len(repos))
	sem := make(chan struct{}, 10)

	for i, repo := range repos {
		i, repo := i, repo
		go func() {
			sem <- struct{}{}
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			infos[i] = collectBranchInfo(repo, target, fetchRemote)
		}()
	}

	for range repos {
		<-done
	}

	printBranchesTable(infos, target)
	return nil
}

func printBranchesHeader(target string, fetchRemote bool, n int) {
	switch {
	case target != "" && fetchRemote:
		fmt.Printf("Fetching from origin and collecting branch info for %q across %d repositories…\n\n", target, n)
	case target != "":
		fmt.Printf("Collecting branch info for %q across %d repositories…\n\n", target, n)
	case fetchRemote:
		fmt.Printf("Fetching from origin and collecting branch info for %d repositories…\n\n", n)
	default:
		fmt.Printf("Collecting branch info for %d repositories…\n\n", n)
	}
}

func collectBranchInfo(repo *db.Repository, target string, fetchRemote bool) branchInfo {
	info := branchInfo{name: repo.Alias}

	if fetchRemote {
		// Best-effort: a fetch failure (offline, missing remote) must not abort
		// the dashboard — fall back to the last-known remote-tracking refs.
		//nolint:errcheck // fetch failure intentionally falls back to cached refs
		_ = git.Fetch(repo.Path)
	}

	current, err := git.CurrentBranch(repo.Path)
	if err != nil {
		info.err = err.Error()
		return info
	}
	info.current = current

	// Resolve the default branch to a concrete ref for the merged-into check,
	// preferring the local branch and falling back to its remote-tracking ref.
	defaultRef := "refs/heads/" + repo.DefaultBranch
	if !git.BranchExists(repo.Path, defaultRef) {
		defaultRef = "refs/remotes/origin/" + repo.DefaultBranch
	}

	if target == "" {
		// Subject is the branch the repo is currently on.
		info.hasSubject = true
		//nolint:errcheck // a lookup failure degrades to an empty upstream, not a hard error
		info.upstream, _ = git.Upstream(repo.Path, current)
		//nolint:errcheck // a lookup failure degrades to 0/0 ahead/behind, not a hard error
		info.ahead, info.behind, _ = git.AheadBehindOf(repo.Path, current)
		info.merged = mergedStateFor(repo.Path, current == repo.DefaultBranch, "HEAD", defaultRef)
		return info
	}

	// Subject is the target branch.
	localOK := git.BranchExists(repo.Path, "refs/heads/"+target)
	remoteOK := git.BranchExists(repo.Path, "refs/remotes/origin/"+target)
	info.targetState = existenceLabel(localOK, remoteOK)
	info.onTarget = current == target

	if localOK {
		// Upstream and ahead/behind need refs/heads/<target>@{upstream}, which only
		// exists when the target is checked out locally.
		info.hasSubject = true
		//nolint:errcheck // a lookup failure degrades to an empty upstream, not a hard error
		info.upstream, _ = git.Upstream(repo.Path, target)
		//nolint:errcheck // a lookup failure degrades to 0/0 ahead/behind, not a hard error
		info.ahead, info.behind, _ = git.AheadBehindOf(repo.Path, target)
	}

	if localOK || remoteOK {
		subjRef := "refs/heads/" + target
		if !localOK {
			subjRef = "refs/remotes/origin/" + target
		}
		info.merged = mergedStateFor(repo.Path, target == repo.DefaultBranch, subjRef, defaultRef)
	}
	// A target missing both locally and on origin leaves merged as mergedUnknown.

	return info
}

func mergedStateFor(path string, isDefault bool, subjRef, defaultRef string) mergedState {
	if isDefault {
		return mergedDefault
	}
	merged, err := git.BranchMergedInto(path, subjRef, defaultRef)
	if err != nil {
		return mergedUnknown
	}
	if merged {
		return mergedYes
	}
	return mergedNo
}

func existenceLabel(localOK, remoteOK bool) string {
	switch {
	case localOK && remoteOK:
		return "local+remote"
	case localOK:
		return "local only"
	case remoteOK:
		return "remote only"
	default:
		return "missing"
	}
}

func printBranchesTable(infos []branchInfo, target string) {
	if target == "" {
		printBranchesCurrent(infos)
		return
	}
	printBranchesTarget(infos, target)
}

func printBranchesCurrent(infos []branchInfo) {
	hdr := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)

	fmt.Printf("%-22s  %-26s  %-22s  %-14s  %s\n",
		hdr.Sprint("REPO"),
		hdr.Sprint("BRANCH"),
		hdr.Sprint("UPSTREAM"),
		hdr.Sprint("AHEAD/BEHIND"),
		hdr.Sprint("MERGED"),
	)
	fmt.Println(strings.Repeat("─", 92))

	for _, info := range infos {
		if info.err != "" {
			fmt.Printf("%-22s  %s\n", cyan.Sprint(info.name), red.Sprintf("ERROR: %s", info.err))
			continue
		}
		fmt.Printf("%-22s  %-26s  %-22s  %-14s  %s\n",
			cyan.Sprint(info.name),
			truncate(info.current, 26),
			formatUpstream(info),
			formatAheadBehind(info),
			formatMerged(info.merged),
		)
	}
}

func printBranchesTarget(infos []branchInfo, target string) {
	hdr := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)

	fmt.Printf("%-22s  %-18s  %-24s  %-22s  %-14s  %s\n",
		hdr.Sprint("REPO"),
		hdr.Sprint("TARGET"),
		hdr.Sprint("CURRENT"),
		hdr.Sprint("UPSTREAM"),
		hdr.Sprint("AHEAD/BEHIND"),
		hdr.Sprint("MERGED"),
	)
	fmt.Println(strings.Repeat("─", 108))

	for _, info := range infos {
		if info.err != "" {
			fmt.Printf("%-22s  %s\n", cyan.Sprint(info.name), red.Sprintf("ERROR: %s", info.err))
			continue
		}
		fmt.Printf("%-22s  %-18s  %-24s  %-22s  %-14s  %s\n",
			cyan.Sprint(info.name),
			formatTarget(info),
			truncate(info.current, 24),
			formatUpstream(info),
			formatAheadBehind(info),
			formatMerged(info.merged),
		)
	}
}

func formatTarget(info branchInfo) string {
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	marker := "  "
	if info.onTarget {
		marker = green.Sprint("● ")
	}

	var label string
	switch info.targetState {
	case "local+remote":
		label = green.Sprint(info.targetState)
	case "local only", "remote only":
		label = yellow.Sprint(info.targetState)
	default: // missing
		label = red.Sprint(info.targetState)
	}
	return marker + label
}

func formatUpstream(info branchInfo) string {
	dim := color.New(color.FgWhite)
	if !info.hasSubject {
		return dim.Sprint("—")
	}
	if info.upstream == "" {
		return dim.Sprint("none")
	}
	return truncate(info.upstream, 22)
}

func formatAheadBehind(info branchInfo) string {
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.FgWhite)

	switch {
	case !info.hasSubject:
		return dim.Sprint("—")
	case info.behind > 0 && info.ahead > 0:
		return yellow.Sprintf("↓%d ↑%d", info.behind, info.ahead)
	case info.behind > 0:
		return yellow.Sprintf("%d behind", info.behind)
	case info.ahead > 0:
		return dim.Sprintf("%d ahead", info.ahead)
	default:
		return green.Sprint("up to date")
	}
}

func formatMerged(state mergedState) string {
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.FgWhite)

	switch state {
	case mergedYes:
		return green.Sprint("merged")
	case mergedNo:
		return yellow.Sprint("not merged")
	case mergedDefault:
		return dim.Sprint("(default)")
	default:
		return dim.Sprint("—")
	}
}

// truncate shortens s to at most max characters, adding an ellipsis when cut.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
