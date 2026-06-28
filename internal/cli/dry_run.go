package cli

import (
	"fmt"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
)

type dryRunItem struct {
	repo       *db.Repository
	actions    []string
	skipReason string
	warning    string
}

func printDryRunPreview(title string, items []dryRunItem) {
	fmt.Println("DRY RUN: no changes made")
	if title != "" {
		fmt.Println()
		fmt.Println(title)
	}

	for _, item := range items {
		alias := "(unknown)"
		path := ""
		if item.repo != nil {
			alias = item.repo.Alias
			path = item.repo.Path
		}

		fmt.Printf("\n[%s]", alias)
		if path != "" {
			fmt.Printf(" %s", path)
		}
		fmt.Println()

		if item.skipReason != "" {
			fmt.Printf("  SKIP: %s\n", item.skipReason)
			continue
		}
		if item.warning != "" {
			fmt.Printf("  WARNING: %s\n", item.warning)
		}
		if len(item.actions) == 0 {
			fmt.Println("  No action needed.")
			continue
		}
		fmt.Println("  Would run:")
		for _, action := range item.actions {
			fmt.Printf("    - %s\n", action)
		}
	}

	fmt.Println()
	fmt.Println("No changes made.")
}

func porcelainPath(line string) string {
	if len(line) > 3 {
		return strings.TrimSpace(line[3:])
	}
	return strings.TrimSpace(line)
}

func discardDryRunActions(porcelainFiles []string) []string {
	var staged []string
	var tracked []string
	var untracked []string

	for _, line := range porcelainFiles {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		path := porcelainPath(line)
		if path == "" {
			continue
		}

		switch {
		case status == "??":
			untracked = append(untracked, path)
		case status[0] == 'A':
			staged = append(staged, path)
		default:
			tracked = append(tracked, path)
		}
	}

	var actions []string
	if len(staged) > 0 {
		joined := strings.Join(staged, " ")
		actions = append(actions, "git reset HEAD -- "+joined)
		actions = append(actions, "git clean -fd -- "+joined)
	}
	if len(tracked) > 0 {
		joined := strings.Join(tracked, " ")
		actions = append(actions, "git reset HEAD -- "+joined)
		actions = append(actions, "git checkout -- "+joined)
	}
	if len(untracked) > 0 {
		actions = append(actions, "git clean -fd -- "+strings.Join(untracked, " "))
	}
	return actions
}
