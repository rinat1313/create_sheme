package service

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeTreeFilter(t *testing.T) {
	got, err := NormalizeTreeFilter("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "all" || got.NodeXMLIDs == nil || len(got.NodeXMLIDs) != 0 {
		t.Fatalf("default: %+v", got)
	}
	got, err = NormalizeTreeFilter("schemes", []string{"a", "", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "schemes" || len(got.NodeXMLIDs) != 2 || got.NodeXMLIDs[0] != "a" || got.NodeXMLIDs[1] != "b" {
		t.Fatalf("dedupe: %+v", got)
	}
	if _, err = NormalizeTreeFilter("exclude", nil); err != ErrInvalidTreeFilterMode {
		t.Fatalf("want invalid mode, got %v", err)
	}
	if _, err = NormalizeTreeFilter("projects", nil); err != ErrInvalidTreeFilterMode {
		t.Fatalf("want invalid projects, got %v", err)
	}
}

func TestExpandTreeFilterSchemeIDs(t *testing.T) {
	folderID := uuidMust("11111111-1111-1111-1111-111111111111")
	projectID := uuidMust("22222222-2222-2222-2222-222222222222")
	schemeAID := uuidMust("33333333-3333-3333-3333-333333333333")
	schemeBID := uuidMust("44444444-4444-4444-4444-444444444444")
	nodes := []treeFilterNode{
		{ID: folderID, Type: "folder", XMLID: "fld"},
		{ID: projectID, ParentID: &folderID, Type: "project", XMLID: "prj"},
		{ID: schemeAID, ParentID: &projectID, Type: "graphic_scheme", XMLID: "sch-a"},
		{ID: schemeBID, ParentID: &projectID, Type: "graphic_scheme", XMLID: "sch-b"},
	}
	got := ExpandTreeFilterSchemeIDs(nodes, []string{"fld"})
	if len(got) != 2 || got[0] != "sch-a" || got[1] != "sch-b" {
		t.Fatalf("folder expand: %v", got)
	}
	got = ExpandTreeFilterSchemeIDs(nodes, []string{"sch-a", "missing"})
	if len(got) != 1 || got[0] != "sch-a" {
		t.Fatalf("scheme keep: %v", got)
	}
}

func uuidMust(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
