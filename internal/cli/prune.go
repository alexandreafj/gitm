package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

type pruneOptions struct {
	yes         bool
	force       bool
	noFetch     bool
	repoAliases []string
	groupName   string
}

func pruneCmd() *cobra.Command {
	var opts pruneOptions

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete local branches that are already merged",
		Long: `List the local branches of every registered repository and delete the ones
that are already merged.

By default this is a dry run: it shows every local branch and what would
happen to it, and changes nothing. Pass --yes to delete.

First it runs git fetch --prune, so branches deleted on the remote are seen.
Then each local branch gets a status:

  merged         every commit is in origin/<default branch>      deleted
  upstream gone  the remote branch was deleted, usually after a
                 squash or rebase merge on GitHub                 deleted
  not pushed     no upstream, local-only work                    kept, always
  current        checked out                                     kept
  default        the default branch itself                       kept, always
  N ahead        commits not pushed to the upstream              kept, unless --force
  not merged     pushed, but not merged yet                      kept

A squash merge never makes the branch commits part of the default branch, so
the merged check cannot see it. The gone upstream is the signal instead. For
those branches, the commits that are not in the default branch are listed
under the table. Check them before you pass --yes.

The merged check uses origin/<default branch>, and falls back to the local
default branch when there is no remote ref.

Use --no-fetch to skip git fetch --prune and use the refs that are already
local. Use --repo / -r to limit to specific repositories by alias. Use
--group / -g to limit to repositories in a group.`,
		Example: `  gitm prune
  gitm prune --yes
  gitm prune -r api-gateway,auth-service
  gitm prune -g backend --yes
  gitm prune --yes --force`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(os.Stdout, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Delete the branches (without it, only show what would be deleted)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Also delete branches with commits not pushed to their upstream")
	cmd.Flags().BoolVar(&opts.noFetch, "no-fetch", false, "Skip git fetch --prune and use the refs that are already local")
	cmd.Flags().StringSliceVarP(&opts.repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &opts.groupName)
	return cmd
}

type pruneStatus int

const (
	pruneMerged pruneStatus = iota
	pruneGone
	pruneAhead
	pruneNotMerged
	pruneNotPushed
	pruneCurrent
	pruneDefault
)

type pruneBranch struct {
	git.LocalBranch
	status pruneStatus
	// commits not in the default branch, listed for gone branches only
	commits []string
}

func (b pruneBranch) deletable(force bool) bool {
	return b.status == pruneMerged || b.status == pruneGone || (force && b.status == pruneAhead)
}

type pruneRepo struct {
	repo     *db.Repository
	branches []pruneBranch
	err      error
}

func runPrune(w io.Writer, opts pruneOptions) error {
	repos, err := resolveReposWithGroup(opts.repoAliases, opts.groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Fprintln(w, noReposMessage(opts.repoAliases, opts.groupName))
		return nil
	}
	if err := reconcileDefaultBranches(database, repos, true); err != nil {
		return fmt.Errorf("refresh default branches: %w", err)
	}

	results := make([]pruneRepo, len(repos))
	done := make(chan struct{}, len(repos))
	sem := make(chan struct{}, 10)
	for i, repo := range repos {
		go func() {
			sem <- struct{}{}
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			results[i] = collectPrune(repo, !opts.noFetch)
		}()
	}
	for range repos {
		<-done
	}

	if !opts.yes {
		fmt.Fprintln(w, "DRY RUN: no changes made")
		fmt.Fprintln(w)
	}
	if err := printPruneTable(w, results, opts.force); err != nil {
		return err
	}
	printGoneCommits(w, results)

	failed := 0
	var toDelete []*db.Repository
	branchCount := 0
	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		n := 0
		for _, b := range r.branches {
			if b.deletable(opts.force) {
				n++
			}
		}
		if n > 0 {
			toDelete = append(toDelete, r.repo)
			branchCount += n
		}
	}

	fmt.Fprintln(w)
	switch {
	case branchCount == 0:
		fmt.Fprintln(w, "Nothing to delete.")
	case !opts.yes:
		fmt.Fprintf(w, "%d branch(es) would be deleted in %d repository(ies). Run with --yes to delete.\n", branchCount, len(toDelete))
	default:
		fmt.Fprintf(w, "Deleting %d branch(es) in %d repository(ies)…\n\n", branchCount, len(toDelete))
		byRepo := make(map[*db.Repository]pruneRepo, len(results))
		for _, r := range results {
			byRepo[r.repo] = r
		}
		deleted := runner.Run(toDelete, func(repo *db.Repository) (string, string, error) {
			return deletePruneBranches(repo, byRepo[repo].branches, opts.force)
		})
		if runner.HasErrors(deleted) {
			failed += runner.ErrorCount(deleted)
		}
	}

	if failed > 0 {
		return fmt.Errorf("prune failed for %d repository(ies)", failed)
	}
	return nil
}

func collectPrune(repo *db.Repository, fetch bool) pruneRepo {
	result := pruneRepo{repo: repo}
	if fetch {
		if err := git.FetchPrune(repo.Path); err != nil {
			result.err = fmt.Errorf("fetch --prune: %w", err)
			return result
		}
	}

	defaultRef := "refs/remotes/origin/" + repo.DefaultBranch
	if !git.BranchExists(repo.Path, defaultRef) {
		defaultRef = "refs/heads/" + repo.DefaultBranch
		if !git.BranchExists(repo.Path, defaultRef) {
			result.err = fmt.Errorf("default branch %q not found locally or on origin", repo.DefaultBranch)
			return result
		}
	}

	current, err := git.CurrentBranch(repo.Path)
	if err != nil {
		result.err = fmt.Errorf("read current branch: %w", err)
		return result
	}
	branches, err := git.LocalBranches(repo.Path)
	if err != nil {
		result.err = err
		return result
	}

	for _, lb := range branches {
		b, err := classifyPruneBranch(repo.Path, lb, current, repo.DefaultBranch, defaultRef)
		if err != nil {
			result.err = err
			return result
		}
		result.branches = append(result.branches, b)
	}
	return result
}

// classifyPruneBranch gives a branch its status. The order matters: the
// keep-always checks run before the merged check, so a branch that was never
// pushed is kept even when it has no commits of its own yet.
func classifyPruneBranch(path string, lb git.LocalBranch, current, defaultBranch, defaultRef string) (pruneBranch, error) {
	b := pruneBranch{LocalBranch: lb}
	switch {
	case lb.Name == defaultBranch:
		b.status = pruneDefault
		return b, nil
	case lb.Name == current:
		b.status = pruneCurrent
		return b, nil
	case lb.Upstream == "":
		b.status = pruneNotPushed
		return b, nil
	}

	merged, err := git.BranchMergedInto(path, "refs/heads/"+lb.Name, defaultRef)
	if err != nil {
		return b, fmt.Errorf("check if %s is merged: %w", lb.Name, err)
	}
	switch {
	case merged:
		b.status = pruneMerged
	case lb.Gone:
		b.status = pruneGone
		b.commits, err = git.CommitsNotIn(path, lb.Name, defaultRef)
		if err != nil {
			return b, err
		}
	case lb.Ahead > 0:
		b.status = pruneAhead
	default:
		b.status = pruneNotMerged
	}
	return b, nil
}

// deletePruneBranches uses git branch -D. The safe -d checks against HEAD or
// the upstream, not the default branch, so it refuses squash-merged branches
// and merged branches while another branch is checked out. classifyPruneBranch
// is the safety check.
func deletePruneBranches(repo *db.Repository, branches []pruneBranch, force bool) (string, string, error) {
	var deleted []string
	for _, b := range branches {
		if !b.deletable(force) {
			continue
		}
		if err := git.DeleteLocalBranch(repo.Path, b.Name, true); err != nil {
			return "", "", fmt.Errorf("delete %s (deleted so far: %s): %w", b.Name, strings.Join(deleted, ", "), err)
		}
		deleted = append(deleted, b.Name)
	}
	return fmt.Sprintf("deleted %d branch(es)\n%s", len(deleted), strings.Join(deleted, "\n")), "", nil
}

func printPruneTable(w io.Writer, results []pruneRepo, force bool) error {
	tw := newTable(w)
	headerRow(tw, "REPO", "BRANCH", "STATUS", "LAST COMMIT", "ACTION")
	for _, r := range results {
		alias := cell(aliasColor, r.repo.Alias)
		if r.err != nil {
			row(tw, alias, dash(), dash(), dash(), cell(errColor, fmt.Sprintf("ERROR: %v", r.err)))
			continue
		}
		for _, b := range r.branches {
			row(tw, alias, truncate(b.Name, 40), formatPruneStatus(b), cell(dimColor, b.LastCommit), formatPruneAction(b, force))
		}
	}
	return flushTable(tw, "prune")
}

func formatPruneStatus(b pruneBranch) string {
	switch b.status {
	case pruneMerged:
		return cell(okColor, "merged")
	case pruneGone:
		return cell(okColor, "upstream gone")
	case pruneAhead:
		return cell(warnColor, fmt.Sprintf("%d ahead", b.Ahead))
	case pruneNotMerged:
		return cell(dimColor, "not merged")
	case pruneNotPushed:
		return cell(warnColor, "not pushed")
	case pruneCurrent:
		return cell(dimColor, "current")
	default:
		return cell(dimColor, "default")
	}
}

func formatPruneAction(b pruneBranch, force bool) string {
	switch {
	case b.deletable(force):
		return cell(errColor, "delete")
	case b.status == pruneAhead:
		return cell(dimColor, "keep (--force deletes)")
	default:
		return cell(dimColor, "keep")
	}
}

// printGoneCommits lists the commits of gone branches. The table cannot show
// them inline without stretching the BRANCH column to the longest subject.
func printGoneCommits(w io.Writer, results []pruneRepo) {
	header := false
	for _, r := range results {
		for _, b := range r.branches {
			if b.status != pruneGone || len(b.commits) == 0 {
				continue
			}
			if !header {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Upstream gone. These commits are not in the default branch, check them before --yes:")
				header = true
			}
			fmt.Fprintf(w, "\n  %s %s\n", color.CyanString(r.repo.Alias), b.Name)
			for _, c := range b.commits {
				fmt.Fprintf(w, "    %s\n", c)
			}
		}
	}
}
