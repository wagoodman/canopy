package db

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestDedupeReferences simulates a DB created by the pre-uniqueIndex schema (duplicate
// references allowed) and verifies dedupeReferences collapses them and repoints events onto
// the surviving row, so the idx_ref_identity unique index can be added on migration.
func TestDedupeReferences(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:deduptest?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// legacy references table WITHOUT the composite unique index, plus a minimal test_events table
	require.NoError(t, gdb.Exec(`CREATE TABLE "references" (id INTEGER PRIMARY KEY, package TEXT, function TEXT, t_run_name TEXT)`).Error)
	require.NoError(t, gdb.Exec(`CREATE TABLE test_events (id INTEGER PRIMARY KEY, reference_id INTEGER)`).Error)

	// ids 1 and 2 are duplicates of the same identity; id 3 is distinct
	require.NoError(t, gdb.Exec(`INSERT INTO "references" (id,package,function,t_run_name) VALUES (1,'p','TestA',''),(2,'p','TestA',''),(3,'p','TestB','')`).Error)
	require.NoError(t, gdb.Exec(`INSERT INTO test_events (id,reference_id) VALUES (10,1),(11,2),(12,3)`).Error)

	require.NoError(t, dedupeReferences(gdb))

	// only the two survivors remain (keeper id 1 for TestA, id 3 for TestB)
	var refCount int64
	require.NoError(t, gdb.Table("references").Count(&refCount).Error)
	require.Equal(t, int64(2), refCount)

	// the duplicate row (id 2) is gone
	var dupExists int64
	require.NoError(t, gdb.Table("references").Where("id = ?", 2).Count(&dupExists).Error)
	require.Equal(t, int64(0), dupExists)

	// the event that pointed at the duplicate is repointed to the keeper
	var refID int64
	require.NoError(t, gdb.Raw(`SELECT reference_id FROM test_events WHERE id = 11`).Scan(&refID).Error)
	require.Equal(t, int64(1), refID)

	// the distinct reference's event is untouched
	require.NoError(t, gdb.Raw(`SELECT reference_id FROM test_events WHERE id = 12`).Scan(&refID).Error)
	require.Equal(t, int64(3), refID)
}
