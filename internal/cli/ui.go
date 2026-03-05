package cli

import (
	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/tui"
)

type ui interface {
	MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error)
	FileSelect(porcelainLines []string, title string) ([]string, error)
	CommitMessageInput(repoAlias string) (string, error)
	BranchNameInput() (string, error)
}

type liveUI struct{}

func (liveUI) MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error) {
	return tui.MultiSelect(repos, title, preSelectAll, disabledIdxs)
}

func (liveUI) FileSelect(porcelainLines []string, title string) ([]string, error) {
	return tui.FileSelect(porcelainLines, title)
}

func (liveUI) CommitMessageInput(repoAlias string) (string, error) {
	return tui.CommitMessageInput(repoAlias)
}

func (liveUI) BranchNameInput() (string, error) {
	return tui.BranchNameInput()
}
