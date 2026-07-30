package migrations

import (
	iofs "io/fs"
	"strings"
	"testing"
)

func TestInitialSchemaIsCanonical(t *testing.T) {
	entries, err := iofs.ReadDir(fs, "sql")
	if err != nil {
		t.Fatal(err)
	}
	var migrationNames []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrationNames = append(migrationNames, entry.Name())
		}
	}
	if len(migrationNames) != 1 || migrationNames[0] != "000001_initialize.sql" {
		t.Fatalf("migrations = %v, want one canonical initial schema", migrationNames)
	}
	schema, err := iofs.ReadFile(fs, "sql/000001_initialize.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, obsolete := range []string{
		"base_version", "published_version", " version BIGINT", "curriculum_unit_ids",
		"restore_unit", "curriculum_unit_restorations", "reverts_proposal_id",
	} {
		if strings.Contains(string(schema), obsolete) {
			t.Errorf("initial schema retains obsolete version field %q", obsolete)
		}
	}
}
