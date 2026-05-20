package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/tui"
)

type ui interface {
	MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error)
	FileSelect(porcelainLines []string, title string) ([]string, error)
	CommitMessageInput(repoAlias, branchName string) (string, error)
	BranchNameInput() (string, error)
	Confirm(prompt string) (bool, error)
}

type liveUI struct{}

func (liveUI) MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error) {
	return tui.MultiSelect(repos, title, preSelectAll, disabledIdxs)
}

func (liveUI) FileSelect(porcelainLines []string, title string) ([]string, error) {
	return tui.FileSelect(porcelainLines, title)
}

func (liveUI) CommitMessageInput(repoAlias, branchName string) (string, error) {
	return tui.CommitMessageInput(repoAlias, branchName)
}

func (liveUI) BranchNameInput() (string, error) {
	return tui.BranchNameInput()
}

// Confirm prints a yes/no prompt and reports whether the user answered yes.
// Anything other than "y"/"yes" (case-insensitive) is treated as no.
func (liveUI) Confirm(prompt string) (bool, error) {
	fmt.Print(prompt + " ")
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}
