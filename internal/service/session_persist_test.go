package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func seedTwoSchemeSolution(t *testing.T, store *repository.Store) (uuid.UUID, domain.GraphicObject, domain.GraphicObject) {
	t.Helper()
	ctx := context.Background()
	solutionID := uuid.New()
	objA := domain.GraphicObject{
		Id:   "obj-a",
		Tag:  "TAG-A",
		Name: "ObjA",
		TechnologyObject: []domain.TechnologyObject{
			{Id: "obj-a", Tag: "TAG-A", Template: "Pump", Name: "ObjA"},
		},
	}
	objB := domain.GraphicObject{
		Id:   "obj-b",
		Tag:  "TAG-B",
		Name: "ObjB",
		TechnologyObject: []domain.TechnologyObject{
			{Id: "obj-b", Tag: "TAG-B", Template: "Pump", Name: "ObjB"},
		},
	}
	schemeA := domain.GraphicScheme{
		Id:            "sch-a",
		Name:          "SchemeA",
		Template:      "T",
		GraphicObject: []domain.GraphicObject{objA},
	}
	schemeB := domain.GraphicScheme{
		Id:            "sch-b",
		Name:          "SchemeB",
		Template:      "T",
		GraphicObject: []domain.GraphicObject{objB},
	}
	payloadA, err := json.Marshal(schemeA)
	if err != nil {
		t.Fatalf("marshal scheme A: %v", err)
	}
	payloadB, err := json.Marshal(schemeB)
	if err != nil {
		t.Fatalf("marshal scheme B: %v", err)
	}
	lean := domain.Solution{
		Name:       "session-persist",
		DataLayout: domain.LeanSchemesLayoutV1,
		Projects: domain.Projects{
			Project: []domain.Project{{
				Id:   "proj-1",
				Name: "P1",
				GraphicScheme: []domain.GraphicScheme{
					domain.SchemeStub(schemeA),
					domain.SchemeStub(schemeB),
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
		Name:         "session-persist",
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
		XMLID:        "sch-a",
		ProjectXMLID: "proj-1",
		Name:         "SchemeA",
		Template:     "T",
		Payload:      payloadA,
	}); err != nil {
		t.Fatalf("InsertScheme A: %v", err)
	}
	if err := store.InsertScheme(ctx, repository.SchemeRow{
		ID:           uuid.New(),
		SolutionID:   solutionID,
		XMLID:        "sch-b",
		ProjectXMLID: "proj-1",
		Name:         "SchemeB",
		Template:     "T",
		Payload:      payloadB,
	}); err != nil {
		t.Fatalf("InsertScheme B: %v", err)
	}
	return solutionID, objA, objB
}

func TestSavePatchOpenPreservesSchemeWorking(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, objA, _ := seedTwoSchemeSolution(t, store)

	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := svc.OpenScheme(ctx, sessionID, "sch-a"); err != nil {
		t.Fatalf("OpenScheme A: %v", err)
	}
	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	patched := objA
	patched.Tag = "TAG-A-EDIT"
	if len(patched.TechnologyObject) > 0 {
		patched.TechnologyObject[0].Tag = "TAG-A-EDIT"
	}
	patch, err := json.Marshal(patched)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}
	if err := svc.PatchObject(ctx, sessionID, "sch-a", "graphic_object", objA.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}

	if err := svc.OpenScheme(ctx, sessionID, "sch-b"); err != nil {
		t.Fatalf("OpenScheme B: %v", err)
	}

	rawA, err := svc.GetScheme(ctx, solutionID, "sch-a", &sessionID)
	if err != nil {
		t.Fatalf("GetScheme A: %v", err)
	}
	var schemeA domain.GraphicScheme
	if err := json.Unmarshal(rawA, &schemeA); err != nil {
		t.Fatalf("unmarshal scheme A: %v", err)
	}
	if len(schemeA.GraphicObject) == 0 || schemeA.GraphicObject[0].Tag != "TAG-A-EDIT" {
		t.Fatalf("scheme A tag after open B = %q want TAG-A-EDIT", schemeA.GraphicObject[0].Tag)
	}

	working, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges: %v", err)
	}
	foundWorking := false
	for _, ch := range working {
		if ch.EntityXMLID == objA.Id && ch.SchemeXMLID == "sch-a" {
			foundWorking = true
			break
		}
	}
	if !foundWorking {
		t.Fatal("expected working change for scheme A after OpenScheme B")
	}

	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	workingAfter, err := store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		t.Fatalf("ListWorkingChanges after save: %v", err)
	}
	if len(workingAfter) != 0 {
		t.Fatalf("working should be empty after Save, got %d", len(workingAfter))
	}

	rawSaved, err := svc.GetScheme(ctx, solutionID, "sch-a", nil)
	if err != nil {
		t.Fatalf("GetScheme A after save: %v", err)
	}
	var savedA domain.GraphicScheme
	if err := json.Unmarshal(rawSaved, &savedA); err != nil {
		t.Fatalf("unmarshal saved A: %v", err)
	}
	if len(savedA.GraphicObject) == 0 || savedA.GraphicObject[0].Tag != "TAG-A-EDIT" {
		t.Fatalf("saved scheme A tag = %q want TAG-A-EDIT", savedA.GraphicObject[0].Tag)
	}
}

func TestOpenSchemeFlushesOtherSchemeDraft(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, objA, _ := seedTwoSchemeSolution(t, store)

	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := svc.OpenScheme(ctx, sessionID, "sch-a"); err != nil {
		t.Fatalf("OpenScheme A: %v", err)
	}

	patched := objA
	patched.Name = "RenamedA"
	patch, err := json.Marshal(patched)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}
	if err := svc.PatchObject(ctx, sessionID, "sch-a", "graphic_object", objA.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}

	draftsBefore, err := store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListDraftChanges: %v", err)
	}
	if len(draftsBefore) != 1 {
		t.Fatalf("drafts before open B = %d want 1", len(draftsBefore))
	}

	if err := svc.OpenScheme(ctx, sessionID, "sch-b"); err != nil {
		t.Fatalf("OpenScheme B: %v", err)
	}

	draftsAfter, err := store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListDraftChanges after: %v", err)
	}
	if len(draftsAfter) != 0 {
		t.Fatalf("drafts after open B = %d want 0", len(draftsAfter))
	}

	working, err := store.ListWorkingChangesForScheme(ctx, solutionID, "sch-a")
	if err != nil {
		t.Fatalf("ListWorkingChangesForScheme: %v", err)
	}
	if len(working) != 1 {
		t.Fatalf("working for sch-a = %d want 1", len(working))
	}

	rawA, err := svc.GetScheme(ctx, solutionID, "sch-a", &sessionID)
	if err != nil {
		t.Fatalf("GetScheme A: %v", err)
	}
	var schemeA domain.GraphicScheme
	if err := json.Unmarshal(rawA, &schemeA); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(schemeA.GraphicObject) == 0 || schemeA.GraphicObject[0].Name != "RenamedA" {
		t.Fatalf("name = %q want RenamedA", schemeA.GraphicObject[0].Name)
	}
}

func TestSaveFlushesDraftWithoutClose(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, objA, _ := seedTwoSchemeSolution(t, store)

	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := svc.OpenScheme(ctx, sessionID, "sch-a"); err != nil {
		t.Fatalf("OpenScheme: %v", err)
	}

	patched := objA
	patched.Tag = "TAG-SAVE-DRAFT"
	if len(patched.TechnologyObject) > 0 {
		patched.TechnologyObject[0].Tag = "TAG-SAVE-DRAFT"
	}
	patch, err := json.Marshal(patched)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := svc.PatchObject(ctx, sessionID, "sch-a", "graphic_object", objA.Id, patch); err != nil {
		t.Fatalf("PatchObject: %v", err)
	}

	if err := svc.Save(ctx, solutionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	drafts, err := store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListDraftChanges: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("drafts after Save = %d want 0", len(drafts))
	}

	raw, err := svc.GetScheme(ctx, solutionID, "sch-a", nil)
	if err != nil {
		t.Fatalf("GetScheme: %v", err)
	}
	var scheme domain.GraphicScheme
	if err := json.Unmarshal(raw, &scheme); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(scheme.GraphicObject) == 0 || scheme.GraphicObject[0].Tag != "TAG-SAVE-DRAFT" {
		t.Fatalf("tag = %q want TAG-SAVE-DRAFT", scheme.GraphicObject[0].Tag)
	}
}

func TestPatchObjectRejectsEmptySchemeXMLID(t *testing.T) {
	svc := &SolutionService{}
	err := svc.PatchObject(context.Background(), uuid.New(), "", "graphic_object", "obj-1", json.RawMessage(`{}`))
	if !errors.Is(err, ErrSchemeXMLIDRequired) {
		t.Fatalf("got %v want ErrSchemeXMLIDRequired", err)
	}
}
