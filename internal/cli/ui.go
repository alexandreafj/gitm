package cli

import (
	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/tui"
)

type ui interface {
	MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error)
	FileSelect(porcelainLines []string, title string) ([]string, error)
	CommitMessageInput(repoAlias, branchName string) (string, error)
	BranchNameInput() (string, error)
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
