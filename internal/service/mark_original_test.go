package service

import (
	"context"
	"encoding/json"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func seedGraphicObjects(t *testing.T, store *repository.Store, objects []domain.GraphicObject) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	solutionID := uuid.New()
	scheme := domain.GraphicScheme{
		Id:            "sch-1",
		Name:          "Scheme",
		Template:      "T",
		GraphicObject: objects,
	}
	payload, err := json.Marshal(scheme)
	if err != nil {
		t.Fatalf("marshal scheme: %v", err)
	}
	lean := domain.Solution{
		Name:       "mark-original",
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
		Name:         "mark-original",
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
	return solutionID
}

func graphicObj(id, tag string, props []domain.TechnologyProperty) domain.GraphicObject {
	return domain.GraphicObject{
		Id:   id,
		Tag:  tag,
		Name: id,
		TechnologyObject: []domain.TechnologyObject{{
			Id:                 id,
			Tag:                tag,
			Template:           "Pump",
			Name:               id,
			TechnologyProperty: props,
		}},
	}
}

func TestGraphicObjectMainTag(t *testing.T) {
	obj := domain.GraphicObject{Tag: "G", TechnologyObject: []domain.TechnologyObject{{Tag: "T"}}}
	if got := graphicObjectMainTag(obj); got != "T" {
		t.Fatalf("main tag=%q want T", got)
	}
	obj.TechnologyObject[0].Tag = ""
	if got := graphicObjectMainTag(obj); got != "G" {
		t.Fatalf("fallback graphic tag=%q want G", got)
	}
}

func TestMarkOriginalObject_SetsSiblingMainTag(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID := seedGraphicObjects(t, store, []domain.GraphicObject{
		graphicObj("orig", "DUP", nil),
		graphicObj("copy-a", "DUP", nil),
		graphicObj("copy-b", "DUP", []domain.TechnologyProperty{{Id: "p1", Name: "P", Tag: "KEEP"}}),
		graphicObj("other", "UNIQUE", []domain.TechnologyProperty{{Id: "p2", Name: "P", Tag: "DUP"}}),
	})
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	result, err := svc.MarkOriginalObject(ctx, sessionID, solutionID, MarkOriginalRequest{
		ObjectID: "orig", SchemeXMLID: "sch-1",
	})
	if err != nil {
		t.Fatalf("MarkOriginalObject: %v", err)
	}
	if result.ChangedCount != 2 {
		t.Fatalf("changed_count=%d want 2 %#v", result.ChangedCount, result.Changed)
	}

	sol, err := svc.loadSolution(ctx, solutionID)
	if err != nil {
		t.Fatalf("loadSolution: %v", err)
	}
	drafts, err := store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListDraftChanges: %v", err)
	}
	applyAllChanges(&sol, drafts)

	orig, ok := svc.findGraphicObjectInSolution(sol, "sch-1", "orig")
	if !ok || graphicObjectMainTag(*orig) != "DUP" {
		t.Fatalf("original tag changed: %+v", orig)
	}
	copyA, _ := svc.findGraphicObjectInSolution(sol, "sch-1", "copy-a")
	if graphicObjectMainTag(*copyA) != "*" {
		t.Fatalf("copy-a tag=%q", graphicObjectMainTag(*copyA))
	}
	copyB, _ := svc.findGraphicObjectInSolution(sol, "sch-1", "copy-b")
	if graphicObjectMainTag(*copyB) != "*" {
		t.Fatalf("copy-b tag=%q", graphicObjectMainTag(*copyB))
	}
	if copyB.TechnologyObject[0].TechnologyProperty[0].Tag != "KEEP" {
		t.Fatalf("nested tag changed: %q", copyB.TechnologyObject[0].TechnologyProperty[0].Tag)
	}
	other, _ := svc.findGraphicObjectInSolution(sol, "sch-1", "other")
	if graphicObjectMainTag(*other) != "UNIQUE" {
		t.Fatalf("other main tag changed: %q", graphicObjectMainTag(*other))
	}
	if other.TechnologyObject[0].TechnologyProperty[0].Tag != "DUP" {
		t.Fatalf("other property tag changed")
	}

	count, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 2 {
		t.Fatalf("changed count=%d want 2", count)
	}
}

func TestMarkOriginalObject_AlreadyStarStillChanged(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID := seedGraphicObjects(t, store, []domain.GraphicObject{
		graphicObj("orig", "*", nil),
		graphicObj("copy-a", "*", nil),
	})
	sessionID, err := svc.CreateSession(ctx, solutionID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	result, err := svc.MarkOriginalObject(ctx, sessionID, solutionID, MarkOriginalRequest{
		ObjectID: "orig", SchemeXMLID: "sch-1",
	})
	if err != nil {
		t.Fatalf("MarkOriginalObject: %v", err)
	}
	if result.ChangedCount != 1 || result.Changed[0].ObjectID != "copy-a" {
		t.Fatalf("result=%#v", result)
	}
	count, err := svc.CountChangedObjects(ctx, solutionID, &sessionID)
	if err != nil {
		t.Fatalf("CountChangedObjects: %v", err)
	}
	if count != 1 {
		t.Fatalf("changed count=%d want 1", count)
	}
	sol, err := svc.loadSolution(ctx, solutionID)
	if err != nil {
		t.Fatalf("loadSolution: %v", err)
	}
	orig, _ := svc.findGraphicObjectInSolution(sol, "sch-1", "orig")
	if graphicObjectMainTag(*orig) != "*" {
		t.Fatalf("original tag=%q", graphicObjectMainTag(*orig))
	}
}
