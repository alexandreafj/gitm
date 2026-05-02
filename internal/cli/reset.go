package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

// resetMode represents which variant of git reset to perform.
type resetMode int

const (
	resetModeMixed resetMode = iota // default: unstage changes, keep in working tree
	resetModeSoft                   // keep changes staged
	resetModeHard                   // discard all changes (irreversible)
)

// repoResetInfo holds the pre-flight info gathered for one repository.
type repoResetInfo struct {
	repo        *db.Repository
	commits     []string // commits that will be undone (oldest first shown last)
	aheadOrigin int      // how many commits ahead of origin BEFORE reset
	resetRef    string   // e.g. "HEAD~2"
	pushedCount int      // how many of the to-be-undone commits are already on origin
}

func resetCmd() *cobra.Command {
	var (
		soft    bool
		hard    bool
		commits int
	)

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset HEAD in selected repositories (soft / mixed / hard)",
		Long: `Undo the last N commits across selected repositories.

Three modes are available:
  (default)  Mixed reset — moves HEAD back, unstages changes, keeps them in the
             working tree. Safe for re-staging and re-committing.

  --soft     Soft reset — moves HEAD back but keeps all changes staged. Ideal
             for amending or squashing commits without losing your work.

  --hard     Hard reset — moves HEAD back AND discards all staged and working-
             tree changes. IRREVERSIBLE — use with caution.

Before applying, a preview of each affected commit is shown so you can confirm
exactly what will be undone.

If any of the commits to be undone have already been pushed to origin, you will
be offered the option to force-push (--force-with-lease) to clean the remote
history. This rewrites shared history — only do this on branches you own.`,
		Example: `  gitm reset                  # mixed reset, undo last commit
  gitm reset --soft           # soft reset, keep changes staged
  gitm reset --hard           # hard reset, discard all changes
  gitm reset --commits 3      # undo last 3 commits (mixed)
  gitm reset --soft --commits 2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := determineResetMode(soft, hard)
			if err != nil {
				return err
			}
			return runReset(mode, commits)
		},
	}

	cmd.Flags().BoolVar(&soft, "soft", false, "Keep changes staged (git reset --soft)")
	cmd.Flags().BoolVar(&hard, "hard", false, "Discard all changes — irreversible (git reset --hard)")
	cmd.Flags().IntVar(&commits, "commits", 1, "Number of commits to undo (default: 1)")
	cmd.MarkFlagsMutuallyExclusive("soft", "hard")

	return cmd
}

// determineResetMode maps the CLI flags to a resetMode value.
// --soft and --hard are mutually exclusive (enforced by cobra above, but also
// validated here so the function can be unit-tested in isolation).
func determineResetMode(soft, hard bool) (resetMode, error) {
	if soft && hard {
		return resetModeMixed, fmt.Errorf("--soft and --hard are mutually exclusive")
	}
	switch {
	case soft:
		return resetModeSoft, nil
	case hard:
		return resetModeHard, nil
	default:
		return resetModeMixed, nil
	}
}

// resetModeName returns a human-readable label for the mode.
func resetModeName(m resetMode) string {
	switch m {
	case resetModeSoft:
		return "soft"
	case resetModeHard:
		return "hard"
	default:
		return "mixed"
	}
}

// resetModeDescription returns a one-line explanation of what the mode does.
func resetModeDescription(m resetMode) string {
	switch m {
	case resetModeSoft:
		return "HEAD moves back; changes remain staged and ready to re-commit"
	case resetModeHard:
		return "HEAD moves back; all staged and working-tree changes are DISCARDED"
	default:
		return "HEAD moves back; changes are unstaged but kept in the working tree"
	}
}

// runReset is the main entry-point for the reset command.
func runReset(mode resetMode, numCommits int) error {
	return runResetWithUI(liveUI{}, mode, numCommits)
}

func runResetWithUI(ui ui, mode resetMode, numCommits int) error {
	if numCommits < 1 {
		return fmt.Errorf("--commits must be at least 1")
	}

	allRepos, err := database.ListRepositories()
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	resetRef := buildResetRef(numCommits)
	infos, skipped := gatherResetInfo(allRepos, numCommits, resetRef)

	for _, msg := range skipped {
		color.Yellow("  ⚠  %s", msg)
	}
	if len(infos) == 0 {
		color.Yellow("\nNo repositories have enough commits to reset %d commit(s).", numCommits)
		return nil
	}

	printResetPreview(infos, mode, numCommits)

	repos := make([]*db.Repository, len(infos))
	for i, info := range infos {
		repos[i] = info.repo
	}

	chosen, err := ui.MultiSelect(repos, "Select repositories to reset", false, nil)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		fmt.Println("Nothing selected — no changes made.")
		return nil
	}

	// Build a map from repo path → info for quick lookup after selection.
	infoByPath := make(map[string]*repoResetInfo, len(infos))
	for i := range infos {
		infoByPath[infos[i].repo.Path] = &infos[i]
	}

	modeName := resetModeName(mode)
	fmt.Printf("\nApplying %s reset (%s) to %d repository(ies)…\n\n",
		color.CyanString(modeName), resetRef, len(chosen))

	if mode == resetModeHard {
		color.Red("WARNING: Hard reset will permanently discard all working-tree changes.")
	}

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		info := infoByPath[repo.Path]

		switch mode {
		case resetModeSoft:
			err = git.ResetSoft(repo.Path, resetRef)
		case resetModeHard:
			err = git.ResetHard(repo.Path, resetRef)
		default:
			err = git.ResetMixed(repo.Path, resetRef)
		}
		if err != nil {
			return "", "", err
		}

		msg := buildResetResultMessage(info, mode)
		return msg, "", nil
	})

	var pushCandidates []*db.Repository
	for _, r := range results {
		if r.Status != runner.StatusSuccess {
			continue
		}
		info := infoByPath[r.Repo.Path]
		if info != nil && info.pushedCount > 0 {
			pushCandidates = append(pushCandidates, r.Repo)
		}
	}

	if len(pushCandidates) > 0 {
		offerForcePush(pushCandidates, numCommits)
	}

	return nil
}

// buildResetRef constructs the git ref string for N commits back.
func buildResetRef(n int) string {
	if n == 1 {
		return "HEAD~1"
	}
	return fmt.Sprintf("HEAD~%d", n)
}

// gatherResetInfo collects commit logs and ahead/behind counts for each repo.
// Repos that cannot satisfy the reset (fewer commits than requested) are
// returned as skipped messages.
func gatherResetInfo(repos []*db.Repository, numCommits int, resetRef string) ([]repoResetInfo, []string) {
	var infos []repoResetInfo
	var skipped []string

	for _, repo := range repos {
		log, err := git.CommitLog(repo.Path, numCommits)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: could not read commit log: %v", repo.Alias, err))
			continue
		}
		if len(log) < numCommits {
			skipped = append(skipped, fmt.Sprintf(
				"%s: only %d commit(s) in history, cannot reset %d",
				repo.Alias, len(log), numCommits,
			))
			continue
		}

		ahead, _, aheadErr := git.AheadBehind(repo.Path, false)
		if aheadErr != nil {
			skipped = append(skipped, fmt.Sprintf("%s: could not check ahead/behind: %v", repo.Alias, aheadErr))
			continue
		}

		// How many of the commits-to-be-undone are already pushed?
		// If we are N ahead and we're resetting M commits:
		//   pushed = max(0, M - ahead)  i.e. commits beyond what's local-only
		pushedCount := numCommits - ahead
		if pushedCount < 0 {
			pushedCount = 0
		}

		infos = append(infos, repoResetInfo{
			repo:        repo,
			commits:     log,
			aheadOrigin: ahead,
			resetRef:    resetRef,
			pushedCount: pushedCount,
		})
	}
	return infos, skipped
}

// printResetPreview prints a human-readable summary of what will happen.
func printResetPreview(infos []repoResetInfo, mode resetMode, numCommits int) {
	modeName := resetModeName(mode)
	modeDesc := resetModeDescription(mode)

	fmt.Printf("\n%s  %s reset  —  %s\n",
		color.New(color.Bold).Sprint("Mode:"),
		color.CyanString(modeName),
		modeDesc,
	)
	fmt.Printf("%s  %s\n\n",
		color.New(color.Bold).Sprint("Scope:"),
		color.YellowString("last %d commit(s) per repository", numCommits),
	)

	for _, info := range infos {
		remoteNote := ""
		if info.pushedCount > 0 {
			remoteNote = color.RedString("  [%d commit(s) already pushed — remote will need force-push]", info.pushedCount)
		} else if info.aheadOrigin >= numCommits {
			remoteNote = color.GreenString("  [all commits are local-only, remote unaffected]")
		}

		fmt.Printf("  %s%s\n", color.CyanString("%-24s", info.repo.Alias), remoteNote)
		for _, c := range info.commits {
			fmt.Printf("    %s %s\n", color.YellowString("↩"), c)
		}
		fmt.Println()
	}
}

// buildResetResultMessage constructs the success message shown in runner output.
func buildResetResultMessage(info *repoResetInfo, mode resetMode) string {
	if info == nil {
		return fmt.Sprintf("%s reset applied", resetModeName(mode))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s reset — undid %d commit(s):\n", resetModeName(mode), len(info.commits)))
	for _, c := range info.commits {
		sb.WriteString(fmt.Sprintf("       %s %s\n", color.YellowString("↩"), color.New(color.FgWhite).Sprint(c)))
	}

	switch mode {
	case resetModeSoft:
		sb.WriteString(color.GreenString("       changes are staged and ready to re-commit"))
	case resetModeHard:
		sb.WriteString(color.RedString("       all changes discarded from index and working tree"))
	default:
		sb.WriteString(color.YellowString("       changes are unstaged but present in the working tree"))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// offerForcePush prompts the user once and, if confirmed, force-pushes all
// candidate repos in parallel via runner.Run.
func offerForcePush(repos []*db.Repository, undoneCommits int) {
	fmt.Println()
	color.Red("┌─────────────────────────────────────────────────────────────────┐")
	color.Red("│  CAUTION: Remote history rewrite                                │")
	color.Red("│                                                                 │")
	color.Red("│  %d of the reset repo(s) had already-pushed commits undone.     │", len(repos))
	color.Red("│  Force-pushing will rewrite the remote branch history.          │")
	color.Red("│  Anyone who has already pulled those commits will need to        │")
	color.Red("│  hard-reset their local branch. Only do this on branches        │")
	color.Red("│  that you own and no one else is using.                         │")
	color.Red("└─────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	fmt.Printf("The following %s will be force-pushed:\n", color.YellowString("%d repo(s)", len(repos)))
	for _, r := range repos {
		fmt.Printf("  %s  %s\n", color.CyanString("%-22s", r.Alias), r.Path)
	}
	fmt.Println()

	fmt.Print("Force-push to clean remote history? " +
		color.RedString("[y/N]") + " ")

	reader := bufio.NewReader(os.Stdin)
	answer, readErr := reader.ReadString('\n')
	if readErr != nil {
		color.Yellow("Skipped — could not read confirmation input: %v", readErr)
		return
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		color.Yellow("Skipped — remote history unchanged. Your local branch is now behind origin.")
		color.Yellow("Run `git push --force-with-lease` manually when ready.")
		return
	}

	fmt.Printf("\nForce-pushing %d repository(ies)…\n\n", len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		if err := git.ForcePush(repo.Path); err != nil {
			return "", "", fmt.Errorf("force-push failed: %w", err)
		}
		branch, branchErr := git.CurrentBranch(repo.Path)
		if branchErr != nil {
			return "", "", fmt.Errorf("get branch: %w", branchErr)
		}
		return fmt.Sprintf("force-pushed branch %s to origin", color.CyanString(branch)), "", nil
	})
}
