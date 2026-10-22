package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testDatabaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://solution:solution@localhost:5432/solution?sslmode=disable"
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "migrations", "001_init.sql")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with migrations/ not found")
		}
		dir = parent
	}
}

func openMigratedTestStore(t *testing.T) *Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	store, err := New(ctx, testDatabaseURL())
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(store.Close)

	root := findRepoRoot(t)
	files := []string{
		"migrations/001_init.sql",
		"migrations/002_compliance_rules.sql",
		"migrations/003_display_and_templates.sql",
		"migrations/004_compliance_value_source.sql",
		"migrations/005_compliance_parameter_scope.sql",
		"migrations/006_tag_edit_rules.sql",
		"migrations/007_tag_edit_jobs.sql",
		"migrations/008_compliance_duplicate.sql",
		"migrations/009_solution_metadata.sql",
		"migrations/010_solution_indexes.sql",
		"migrations/011_users_and_settings.sql",
		"migrations/012_import_error.sql",
		"migrations/013_template_catalog.sql",
		"migrations/014_export_change_acks.sql",
		"migrations/015_compliance_duplicate_background.sql",
		"migrations/016_solution_objects_search.sql",
		"migrations/017_solution_data_jsonb.sql",
		"migrations/018_drop_solution_xml.sql",
		"migrations/019_lean_solution_data.sql",
		"migrations/020_blob_storage.sql",
		"migrations/021_solution_objects_technology_id.sql",
		"migrations/022_solution_changed_objects.sql",
		"migrations/023_compliance_duplicate_technology_id.sql",
	}
	for _, rel := range files {
		sqlBytes, readErr := os.ReadFile(filepath.Join(root, rel))
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		if migErr := store.Migrate(ctx, string(sqlBytes)); migErr != nil {
			t.Fatalf("migrate %s: %v", rel, migErr)
		}
	}
	return store
}

func createTestSolution(t *testing.T, store *Store) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	err := store.CreateSolution(ctx, SolutionRow{
		ID:           id,
		Name:         "changed-objects-test",
		ImportStatus: "ready",
		Data:         json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSolution: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DeleteSolution(context.Background(), id)
	})
	return id
}

func TestClearWorkingChanges_PreservesAcksAndDurable(t *testing.T) {
	store := openMigratedTestStore(t)
	ctx := context.Background()
	solutionID := createTestSolution(t, store)

	patch := json.RawMessage(`{"Id":"obj-1","Tag":"T1"}`)
	if err := store.UpsertWorkingChange(ctx, solutionID, "sch-1", "graphic_object", "obj-1", patch); err != nil {
		t.Fatalf("UpsertWorkingChange: %v", err)
	}
	if err := store.AckExportChangedObjects(ctx, solutionID, "graphic_object", []string{"obj-acked"}); err != nil {
		t.Fatalf("AckExportChangedObjects: %v", err)
	}
	if err := store.UpsertSolutionChangedObject(ctx, solutionID, "graphic_object", "obj-durable", "sch-1"); err != nil {
		t.Fatalf("UpsertSolutionChangedObject: %v", err)
	}

	if err := store.ClearWorkingChanges(ctx, solutionID); err != nil {
		t.Fatalf("ClearWorkingChanges: %v", err)
	}

	working, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges: %v", err)
	}
	if len(working) != 0 {
		t.Fatalf("working should be empty, got %d", len(working))
	}

	acks, err := store.ListExportChangeAcks(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListExportChangeAcks: %v", err)
	}
	if len(acks) != 1 || acks[0].EntityXMLID != "obj-acked" {
		t.Fatalf("acks should be preserved, got %+v", acks)
	}

	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	found := map[string]bool{}
	for _, row := range durable {
		found[row.EntityXMLID] = true
	}
	if !found["obj-1"] {
		t.Fatal("durable should keep obj-1 from UpsertWorkingChange")
	}
	if !found["obj-durable"] {
		t.Fatal("durable should keep explicitly upserted obj-durable")
	}
}

func TestMergeWorkingIntoChangedObjects_Idempotent(t *testing.T) {
	store := openMigratedTestStore(t)
	ctx := context.Background()
	solutionID := createTestSolution(t, store)

	_, err := store.pool.Exec(ctx, `
		INSERT INTO working_changes (id, solution_id, scheme_xml_id, entity_type, entity_xml_id, patch)
		VALUES ($1,$2,'sch-1','graphic_object','obj-flush','{"Id":"obj-flush"}')`,
		uuid.New(), solutionID)
	if err != nil {
		t.Fatalf("insert working without upsert: %v", err)
	}

	if err := store.MergeWorkingIntoChangedObjects(ctx, solutionID); err != nil {
		t.Fatalf("MergeWorkingIntoChangedObjects: %v", err)
	}
	if err := store.MergeWorkingIntoChangedObjects(ctx, solutionID); err != nil {
		t.Fatalf("second MergeWorkingIntoChangedObjects: %v", err)
	}

	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 1 || durable[0].EntityXMLID != "obj-flush" {
		t.Fatalf("expected single durable obj-flush, got %+v", durable)
	}
}

func TestFlushDraftToWorking_UpsertsDurable(t *testing.T) {
	store := openMigratedTestStore(t)
	ctx := context.Background()
	solutionID := createTestSolution(t, store)
	sessionID := uuid.New()
	if err := store.CreateSession(ctx, sessionID, solutionID); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	patch := json.RawMessage(`{"Id":"obj-draft","Tag":"T"}`)
	if err := store.UpsertDraftChange(ctx, uuid.New(), sessionID, "sch-1", "graphic_object", "obj-draft", patch); err != nil {
		t.Fatalf("UpsertDraftChange: %v", err)
	}
	if err := store.FlushDraftToWorking(ctx, solutionID, sessionID); err != nil {
		t.Fatalf("FlushDraftToWorking: %v", err)
	}

	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 1 || durable[0].EntityXMLID != "obj-draft" {
		t.Fatalf("flush should upsert durable, got %+v", durable)
	}

	working, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges: %v", err)
	}
	if len(working) != 1 {
		t.Fatalf("working after flush=%d want 1", len(working))
	}
}
