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

func discardDryRunActions(porcelainFiles []string) []string {
	var resetPaths []string
	var checkoutPaths []string
	var cleanPaths []string
	renameSources := make(map[string]struct{})
	for _, line := range porcelainFiles {
		if len(line) < 4 || !strings.Contains(line[:2], "R") {
			continue
		}
		_, sourcePath, renamed := strings.Cut(line[3:], "\x00")
		if renamed && sourcePath != "" {
			renameSources[sourcePath] = struct{}{}
		}
	}

	for _, line := range porcelainFiles {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		currentPath, sourcePath, renamed := strings.Cut(line[3:], "\x00")
		if currentPath == "" {
			continue
		}

		switch {
		case status == "??":
			if _, restoredByRename := renameSources[currentPath]; !restoredByRename {
				cleanPaths = append(cleanPaths, currentPath)
			}
		case strings.Contains(status, "R") && renamed && sourcePath != "":
			resetPaths = append(resetPaths, currentPath, sourcePath)
			checkoutPaths = append(checkoutPaths, sourcePath)
			cleanPaths = append(cleanPaths, currentPath)
		case strings.Contains(status, "C") && renamed && sourcePath != "":
			resetPaths = append(resetPaths, currentPath)
			cleanPaths = append(cleanPaths, currentPath)
		case status[0] == 'A':
			resetPaths = append(resetPaths, currentPath)
			cleanPaths = append(cleanPaths, currentPath)
		default:
			resetPaths = append(resetPaths, currentPath)
			checkoutPaths = append(checkoutPaths, currentPath)
		}
	}

	formatPathspecs := func(paths []string) string {
		formatted := make([]string, 0, len(paths))
		for _, path := range paths {
			if strings.ContainsAny(path, " \t\r\n\\\"'`$&;|<>(){}[]*?!#~") || strings.HasPrefix(path, ":") {
				literal := ":(literal)" + path
				formatted = append(formatted, "'"+strings.ReplaceAll(literal, "'", "'\"'\"'")+"'")
				continue
			}
			formatted = append(formatted, path)
		}
		return strings.Join(formatted, " ")
	}

	var actions []string
	if len(resetPaths) > 0 {
		actions = append(actions, "git reset HEAD -- "+formatPathspecs(resetPaths))
	}
	if len(checkoutPaths) > 0 {
		actions = append(actions, "git checkout -- "+formatPathspecs(checkoutPaths))
	}
	if len(cleanPaths) > 0 {
		actions = append(actions, "git clean -fd -- "+formatPathspecs(cleanPaths))
	}
	return actions
}
