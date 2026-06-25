package db_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

// groupRepositoriesDDL reads the stored CREATE statement for group_repositories
// via a raw connection, so we inspect exactly what the migrations produced.
func groupRepositoriesDDL(t *testing.T, dbPath string) string {
	t.Helper()
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer conn.Close()
	var ddl string
	if err := conn.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='group_repositories'`,
	).Scan(&ddl); err != nil {
		t.Fatalf("read group_repositories schema: %v", err)
	}
	return ddl
}

// The v2 table rebuild used to rename repositories -> repositories_old, which
// SQLite propagated into the group_repositories foreign key before the table was
// dropped, leaving the FK pointing at a non-existent table.
func TestGroupRepositoriesForeignKeyTargetsRepositories(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	ddl := groupRepositoriesDDL(t, dbPath)
	if strings.Contains(ddl, "repositories_old") {
		t.Fatalf("group_repositories FK references dropped table repositories_old:\n%s", ddl)
	}
	if !strings.Contains(ddl, "REFERENCES repositories(") {
		t.Fatalf("group_repositories FK does not reference repositories:\n%s", ddl)
	}
}

// A second startup must not re-run the destructive rebuild and re-corrupt the FK.
func TestGroupRepositoriesForeignKeySurvivesReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reopen.db")

	d1, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := d1.AddRepository("r", "r", "/tmp/r", "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	d1.Close()

	d2, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer d2.Close()

	ddl := groupRepositoriesDDL(t, dbPath)
	if strings.Contains(ddl, "repositories_old") {
		t.Fatalf("group_repositories FK corrupted after reopen:\n%s", ddl)
	}
}
