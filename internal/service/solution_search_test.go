package service

import (
	"testing"

	"create_sheme/internal/domain"

	"github.com/google/uuid"
)

func TestPrepareSearchQuery(t *testing.T) {
	q, idOnly, empty := prepareSearchQuery("  ")
	if !empty || q != "" || idOnly {
		t.Fatalf("empty query: q=%q idOnly=%v empty=%v", q, idOnly, empty)
	}

	q, idOnly, empty = prepareSearchQuery("a")
	if empty || !idOnly || q != "a" {
		t.Fatalf("short query: q=%q idOnly=%v empty=%v", q, idOnly, empty)
	}

	q, idOnly, empty = prepareSearchQuery("ab")
	if empty || idOnly || q != "ab" {
		t.Fatalf("full query: q=%q idOnly=%v empty=%v", q, idOnly, empty)
	}
}

func TestMatchSearchFields(t *testing.T) {
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "pump", false) {
		t.Fatal("expected name match")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "tag-1", false) {
		t.Fatal("expected tag match")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "obj-4", false) {
		t.Fatal("expected id substring match")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "tech-9", false) {
		t.Fatal("expected technology id match")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "val", false) {
		t.Fatal("expected template match")
	}
	if matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "x", true) {
		t.Fatal("short q should not match unrelated id")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "obj", true) {
		t.Fatal("short q should match id prefix")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "tech", true) {
		t.Fatal("short q should match technology id prefix")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "obj-42", true) {
		t.Fatal("short q should match exact id")
	}
	if !matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "tech-9", true) {
		t.Fatal("short q should match exact technology id")
	}
	if matchSearchFields("Pump A", "TAG-1", "obj-42", "tech-9", "Valve", "pump", true) {
		t.Fatal("id-only mode must ignore name")
	}
}

func TestDedupSearchItems(t *testing.T) {
	items := []SolutionSearchItem{
		{ObjectID: "a", Name: "one"},
		{ObjectID: "a", Name: "dup"},
		{ObjectID: "b", Name: "two"},
		{ObjectID: "", Name: "skip"},
	}
	out := dedupSearchItems(items)
	if len(out) != 2 {
		t.Fatalf("expected 2 items, got %d", len(out))
	}
	if out[0].ObjectID != "a" || out[0].Name != "one" {
		t.Fatalf("unexpected first item: %+v", out[0])
	}
	if out[1].ObjectID != "b" {
		t.Fatalf("unexpected second item: %+v", out[1])
	}
}

func TestApplySearchLimit(t *testing.T) {
	items := make([]SolutionSearchItem, 0, 5)
	for i := 0; i < 5; i++ {
		items = append(items, SolutionSearchItem{ObjectID: string(rune('a' + i))})
	}
	page := applySearchLimit(items, 2, 1)
	if len(page) != 2 || page[0].ObjectID != "b" || page[1].ObjectID != "c" {
		t.Fatalf("unexpected page: %+v", page)
	}
	empty := applySearchLimit(items, 10, 10)
	if len(empty) != 0 {
		t.Fatalf("expected empty page, got %d", len(empty))
	}
	def := applySearchLimit(items, 0, -1)
	if len(def) != 5 {
		t.Fatalf("default paging should return all small set, got %d", len(def))
	}
}

func TestBuildSolutionObjectsDisplayHelpers(t *testing.T) {
	sol := domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{
				{
					GraphicScheme: []domain.GraphicScheme{
						{
							Id: "sch1",
							GraphicObject: []domain.GraphicObject{
								{
									Id:   "obj1",
									Name: "GraphicName",
									Tag:  "GraphicTag",
									TechnologyObject: []domain.TechnologyObject{
										{Id: "tech1", Name: "TechName", Tag: "TechTag", Template: "Valve"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	rows := buildSolutionObjects(uuid.New(), sol)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Name != "TechName" {
		t.Fatalf("expected TechName, got %q", rows[0].Name)
	}
	if rows[0].Tag != "TechTag" {
		t.Fatalf("expected TechTag, got %q", rows[0].Tag)
	}
	if rows[0].Template != "Valve" {
		t.Fatalf("expected Valve, got %q", rows[0].Template)
	}
	if rows[0].TechnologyObjectID != "tech1" {
		t.Fatalf("expected tech1, got %q", rows[0].TechnologyObjectID)
	}
}

func TestFilterSearchMatchingAndLimit(t *testing.T) {
	all := []SolutionSearchItem{
		{ObjectID: "id-1", TechnologyObjectID: "to-1", Name: "Alpha", Tag: "T1", ObjectType: "Valve"},
		{ObjectID: "id-1", TechnologyObjectID: "to-1", Name: "Alpha dup", Tag: "T1", ObjectType: "Valve"},
		{ObjectID: "id-2", TechnologyObjectID: "to-2", Name: "Beta", Tag: "T2", ObjectType: "Pump"},
		{ObjectID: "id-3", TechnologyObjectID: "to-3", Name: "Gamma", Tag: "ALPHA", ObjectType: "Pipe"},
		{ObjectID: "zz-9", TechnologyObjectID: "to-z", Name: "Other", Tag: "X", ObjectType: "Valve"},
	}
	matched := make([]SolutionSearchItem, 0)
	for _, item := range all {
		if matchSearchFields(item.Name, item.Tag, item.ObjectID, item.TechnologyObjectID, item.ObjectType, "alpha", false) {
			matched = append(matched, item)
		}
	}
	matched = dedupSearchItems(matched)
	if len(matched) != 2 {
		t.Fatalf("expected 2 matched unique rows, got %d", len(matched))
	}
	page := applySearchLimit(matched, 1, 0)
	if len(page) != 1 || page[0].ObjectID != "id-1" {
		t.Fatalf("unexpected limited page: %+v", page)
	}

	short := make([]SolutionSearchItem, 0)
	for _, item := range all {
		if matchSearchFields(item.Name, item.Tag, item.ObjectID, item.TechnologyObjectID, item.ObjectType, "z", true) {
			short = append(short, item)
		}
	}
	short = dedupSearchItems(short)
	if len(short) != 1 || short[0].ObjectID != "zz-9" {
		t.Fatalf("short id prefix match failed: %+v", short)
	}

	byTech := make([]SolutionSearchItem, 0)
	for _, item := range all {
		if matchSearchFields(item.Name, item.Tag, item.ObjectID, item.TechnologyObjectID, item.ObjectType, "to-2", true) {
			byTech = append(byTech, item)
		}
	}
	if len(byTech) != 1 || byTech[0].ObjectID != "id-2" {
		t.Fatalf("technology id match failed: %+v", byTech)
	}
}
