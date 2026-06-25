package db_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

// Group lookups must report a distinct "group not found" error rather than the
// repository sentinel, so messages don't misleadingly mention repositories.
func TestGroupLookupNotFoundUsesGroupError(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	_, err = d.GetGroup("ghost")
	if !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("GetGroup(ghost) error = %v, want ErrGroupNotFound", err)
	}
	if strings.Contains(err.Error(), "repository") {
		t.Fatalf("group-not-found error mentions repository: %v", err)
	}
}
