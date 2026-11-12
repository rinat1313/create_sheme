package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

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

func openChangedObjectsTestEnv(t *testing.T) (*SolutionService, *repository.Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	store, err := repository.New(ctx, testDatabaseURL())
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
		"migrations/024_tag_edit_rule_selection.sql",
		"migrations/025_tag_edit_rule_check_actions.sql",
		"migrations/026_user_solution_tree_filters.sql",
		"migrations/027_tree_filter_drop_projects.sql",
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
	return NewSolutionService(store, 0), store
}

func seedGraphicSolution(t *testing.T, store *repository.Store, objID, tag string) (uuid.UUID, domain.GraphicObject) {
	t.Helper()
	ctx := context.Background()
	solutionID := uuid.New()
	obj := domain.GraphicObject{
		Id:  objID,
		Tag: tag,
		Name: "Obj",
		TechnologyObject: []domain.TechnologyObject{
			{Id: objID, Tag: tag, Template: "Pump", Name: "Obj"},
		},
	}
	scheme := domain.GraphicScheme{
		Id:            "sch-1",
		Name:          "Scheme",
		Template:      "T",
		GraphicObject: []domain.GraphicObject{obj},
	}
	payload, err := json.Marshal(scheme)
	if err != nil {
		t.Fatalf("marshal scheme: %v", err)
	}
	lean := domain.Solution{
		Name:       "changed-objects",
		DataLayout: domain.LeanSchemesLayoutV1,
		Projects: domain.Projects{
			Project: []domain.Project{{
				Id:   "proj-1",
				Name: "P1",
				GraphicScheme: []domain.GraphicScheme{
					domain.SchemeStub(scheme),
				},
			}},
		},
	}
	leanData, err := json.Marshal(lean)
	if err != nil {
		t.Fatalf("marshal lean: %v", err)
	}
	if err := store.CreateSolution(ctx, repository.SolutionRow{
		ID:           solutionID,
		Name:         "changed-objects",
		ImportStatus: "ready",
		Data:         leanData,
	}); err != nil {
		t.Fatalf("CreateSolution: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DeleteSolution(context.Background(), solutionID)
	})
	if err := store.InsertScheme(ctx, repository.SchemeRow{
		ID:           uuid.New(),
		SolutionID:   solutionID,
		XMLID:        "sch-1",
		ProjectXMLID: "proj-1",
		Name:         "Scheme",
		Template:     "T",
		Payload:      payload,
	}); err != nil {
		t.Fatalf("InsertScheme: %v", err)
	}
	return solutionID, obj
}

func applySemanticWorkingChange(t *testing.T, svc *SolutionService, store *repository.Store, ctx context.Context, solutionID uuid.UUID, schemeXMLID string, obj domain.GraphicObject) {
	t.Helper()
	patch, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}
	if err := store.UpsertWorkingChange(ctx, solutionID, schemeXMLID, "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("UpsertWorkingChange: %v", err)
	}
	if err := svc.applyGraphicObjectChangedMarking(ctx, solutionID, schemeXMLID, obj.Id, patch); err != nil {
		t.Fatalf("applyGraphicObjectChangedMarking: %v", err)
	}
}

func TestSave_PreservesChangedObjectsCount(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-save", "TAG-SAVE")

	patched := obj
	patched.Tag = "TAG-SAVE-EDIT"
	if len(patched.TechnologyObject) > 0 {
		patched.TechnologyObject[0].Tag = "TAG-SAVE-EDIT"
	}
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", patched)

	before, err := svc.CountChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("CountChangedObjects before save: %v", err)
	}
	if before != 1 {
		t.Fatalf("count before save=%d want 1", before)
	}

	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	working, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges: %v", err)
	}
	if len(working) != 0 {
		t.Fatalf("working should be cleared after Save, got %d", len(working))
	}

	after, err := svc.CountChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("CountChangedObjects after save: %v", err)
	}
	if after != 1 {
		t.Fatalf("count after save=%d want 1 (durable preserved)", after)
	}
}

func TestExportChangedOnly_AfterSaveNonEmpty(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-export", "TAG-EXPORT")

	patched := obj
	patched.Tag = "TAG-EXPORT-EDIT"
	if len(patched.TechnologyObject) > 0 {
		patched.TechnologyObject[0].Tag = "TAG-EXPORT-EDIT"
	}
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", patched)
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, name, err := svc.ExportObjects(ctx, solutionID, ExportObjectsRequest{
		Fields:      []string{"Id", "Tag"},
		Format:      "json",
		ChangedOnly: true,
		TagPresence: TagPresenceAll,
	})
	if err != nil {
		t.Fatalf("ExportObjects: %v", err)
	}
	if name == "" {
		t.Fatal("expected export filename")
	}
	var rows []map[string]string
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("unmarshal export: %v body=%s", err, data)
	}
	if len(rows) != 1 || rows[0]["Id"] != obj.Id {
		t.Fatalf("changed_only after Save should be non-empty, got %v", rows)
	}
}

func TestPatchObjectDraft_Save_PreservesChangedCountAndExport(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-draft-save", "TAG-DRAFT")

	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := svc.OpenScheme(ctx, sessionID, "sch-1"); err != nil {
		t.Fatalf("OpenScheme: %v", err)
	}

	patched := obj
	patched.Tag = "TAG-DRAFT-EDIT"
	if len(patched.TechnologyObject) > 0 {
		patched.TechnologyObject[0].Tag = "TAG-DRAFT-EDIT"
	}
	patch, err := json.Marshal(patched)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}
	if err := svc.PatchObject(ctx, sessionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}

	before, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects before save: %v", err)
	}
	if before != 1 {
		t.Fatalf("count before save=%d want 1", before)
	}

	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	working, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges: %v", err)
	}
	if len(working) != 0 {
		t.Fatalf("working should be empty after Save, got %d", len(working))
	}

	after, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects after save: %v", err)
	}
	if after != 1 {
		t.Fatalf("count after draft→Save=%d want 1", after)
	}

	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 1 || durable[0].EntityXMLID != obj.Id {
		t.Fatalf("durable after Save=%+v want obj-draft-save", durable)
	}

	data, _, err := svc.ExportObjects(ctx, solutionID, ExportObjectsRequest{
		Fields:      []string{"Id", "Tag"},
		Format:      "json",
		ChangedOnly: true,
		SessionID:   sessionID.String(),
		TagPresence: TagPresenceAll,
	})
	if err != nil {
		t.Fatalf("ExportObjects: %v", err)
	}
	var rows []map[string]string
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("unmarshal export: %v body=%s", err, data)
	}
	if len(rows) != 1 || rows[0]["Id"] != obj.Id {
		t.Fatalf("changed_only export after draft→Save should be non-empty, got %v", rows)
	}
}

func TestAckChangedObjects_ClearsCountAndDurable(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-ack", "TAG-ACK")

	patchedAck := obj
	patchedAck.Tag = "TAG-ACK"
	if len(patchedAck.TechnologyObject) > 0 {
		patchedAck.TechnologyObject[0].Tag = "TAG-ACK"
	}
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", patchedAck)
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	count, err := svc.AckChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("AckChangedObjects: %v", err)
	}
	if count != 0 {
		t.Fatalf("count after ack=%d want 0", count)
	}

	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 0 {
		t.Fatalf("durable should be empty after ack, got %+v", durable)
	}

	remaining, err := svc.CountChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("count remaining=%d want 0", remaining)
	}
}

func TestReEditAfterAck_ReturnsToCount(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-reedit", "TAG-RE")

	first := obj
	first.Tag = "TAG-RE-1"
	if len(first.TechnologyObject) > 0 {
		first.TechnologyObject[0].Tag = "TAG-RE-1"
	}
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", first)
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := svc.AckChangedObjects(ctx, solutionID, nil); err != nil {
		t.Fatalf("AckChangedObjects: %v", err)
	}

	reedit := obj
	reedit.Tag = "TAG-RE-2"
	if len(reedit.TechnologyObject) > 0 {
		reedit.TechnologyObject[0].Tag = "TAG-RE-2"
	}
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", reedit)

	count, err := svc.CountChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 1 {
		t.Fatalf("re-edit after ack count=%d want 1", count)
	}
}

func TestSemanticMatrix_LayoutOnlyDraftNotCounted(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-layout-draft", "TAG-L")
	obj.AnchorPoint = []domain.AnchorPoint{{X: "0", Y: "0"}}
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	moved := obj
	moved.AnchorPoint = []domain.AnchorPoint{{X: "100", Y: "50"}}
	patch, _ := json.Marshal(moved)
	if err := svc.PatchObject(ctx, sessionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}
	count, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 0 {
		t.Fatalf("layout-only draft count=%d want 0", count)
	}
}

func TestSemanticMatrix_LayoutOnlyWorkingPrunesDurable(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-prune", "TAG-P")
	obj.AnchorPoint = []domain.AnchorPoint{{X: "0", Y: "0"}}
	semantic := obj
	semantic.Tag = "TAG-P-EDIT"
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", semantic)
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}
	layout := obj
	layout.Tag = "TAG-P-EDIT"
	if len(layout.TechnologyObject) > 0 {
		layout.TechnologyObject[0].Tag = "TAG-P-EDIT"
	}
	layout.AnchorPoint = []domain.AnchorPoint{{X: "200", Y: "0"}}
	patch, _ := json.Marshal(layout)
	if err := store.UpsertWorkingChange(ctx, solutionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("UpsertWorkingChange: %v", err)
	}
	count, err := svc.CountChangedObjects(ctx, solutionID, nil)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 0 {
		t.Fatalf("layout-only working count=%d want 0", count)
	}
	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 0 {
		t.Fatalf("durable should be pruned, got %+v", durable)
	}
}

func TestSemanticMatrix_TagChangeCounted(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-tag", "TAG-T")
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	edited := obj
	edited.Tag = "TAG-T-NEW"
	if len(edited.TechnologyObject) > 0 {
		edited.TechnologyObject[0].Tag = "TAG-T-NEW"
	}
	patch, _ := json.Marshal(edited)
	if err := svc.PatchObject(ctx, sessionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}
	count, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 1 {
		t.Fatalf("tag change count=%d want 1", count)
	}
}

func TestSemanticMatrix_LayoutAndTagCounted(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-mix", "TAG-M")
	obj.AnchorPoint = []domain.AnchorPoint{{X: "1", Y: "1"}}
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	mixed := obj
	mixed.AnchorPoint = []domain.AnchorPoint{{X: "99", Y: "99"}}
	mixed.Tag = "TAG-M-NEW"
	if len(mixed.TechnologyObject) > 0 {
		mixed.TechnologyObject[0].Tag = "TAG-M-NEW"
	}
	patch, _ := json.Marshal(mixed)
	if err := svc.PatchObject(ctx, sessionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}
	count, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 1 {
		t.Fatalf("layout+tag count=%d want 1", count)
	}
}

func TestSemanticMatrix_LayoutSaveSkipsDurable(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-save-layout", "TAG-SL")
	obj.AnchorPoint = []domain.AnchorPoint{{X: "0", Y: "0"}}
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := svc.OpenScheme(ctx, sessionID, "sch-1"); err != nil {
		t.Fatalf("OpenScheme: %v", err)
	}
	moved := obj
	moved.AnchorPoint = []domain.AnchorPoint{{X: "50", Y: "50"}}
	patch, _ := json.Marshal(moved)
	if err := svc.PatchObject(ctx, sessionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}
	durable, err := store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListSolutionChangedObjects: %v", err)
	}
	if len(durable) != 0 {
		t.Fatalf("layout-only Save durable=%+v want none", durable)
	}
}

func TestSemanticMatrix_LayoutShiftKeepsExportAck(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, obj := seedGraphicSolution(t, store, "obj-ack-layout", "TAG-AK")
	obj.AnchorPoint = []domain.AnchorPoint{{X: "0", Y: "0"}}
	semantic := obj
	semantic.Tag = "TAG-AK-EDIT"
	applySemanticWorkingChange(t, svc, store, ctx, solutionID, "sch-1", semantic)
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := svc.AckChangedObjects(ctx, solutionID, nil); err != nil {
		t.Fatalf("AckChangedObjects: %v", err)
	}
	layout := obj
	layout.Tag = "TAG-AK-EDIT"
	if len(layout.TechnologyObject) > 0 {
		layout.TechnologyObject[0].Tag = "TAG-AK-EDIT"
	}
	layout.AnchorPoint = []domain.AnchorPoint{{X: "10", Y: "10"}}
	patch, _ := json.Marshal(layout)
	if err := store.UpsertWorkingChange(ctx, solutionID, "sch-1", "graphic_object", obj.Id, patch); err != nil {
		t.Fatalf("UpsertWorkingChange: %v", err)
	}
	acks, err := store.ListExportChangeAcks(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListExportChangeAcks: %v", err)
	}
	if len(acks) != 1 || acks[0].EntityXMLID != obj.Id {
		t.Fatalf("layout shift must keep ack, got %+v", acks)
	}
}
