package service

import (
	"encoding/csv"
	"strings"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"
)

func makeTestSolution(objects []domain.GraphicObject) domain.Solution {
	return domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{
				{
					GraphicScheme: []domain.GraphicScheme{
						{
							Id:            "scheme-1",
							GraphicObject: objects,
						},
					},
				},
			},
		},
	}
}

func objWithTemplate(id, template, tag string) domain.GraphicObject {
	return domain.GraphicObject{
		Id:  id,
		Tag: tag,
		TechnologyObject: []domain.TechnologyObject{
			{Id: id, Tag: tag, Template: template},
		},
	}
}

func TestBuildCSV_PreservesColumnOrder(t *testing.T) {
	fields := []string{"Tag", "Id", "Name"}
	rows := []map[string]string{
		{"Tag": "T1", "Id": "1", "Name": "A"},
		{"Tag": "T2", "Id": "2", "Name": "B"},
	}
	data := buildCSV(fields, rows)
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(records))
	}
	if strings.Join(records[0], ",") != "Tag,Id,Name" {
		t.Fatalf("header order: got %v want Tag,Id,Name", records[0])
	}
	if records[1][0] != "T1" || records[1][1] != "1" || records[1][2] != "A" {
		t.Fatalf("row 1 order: got %v", records[1])
	}
}

func TestMarshalOrderedJSONRows_PreservesColumnOrder(t *testing.T) {
	fields := []string{"Tag", "Id", "Name"}
	rows := []map[string]string{
		{"Tag": "T1", "Id": "1", "Name": "A"},
	}
	data, err := marshalOrderedJSONRows(fields, rows)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	tagIdx := strings.Index(s, `"Tag"`)
	idIdx := strings.Index(s, `"Id"`)
	nameIdx := strings.Index(s, `"Name"`)
	if tagIdx < 0 || idIdx < 0 || nameIdx < 0 {
		t.Fatalf("missing keys in output: %s", s)
	}
	if !(tagIdx < idIdx && idIdx < nameIdx) {
		t.Fatalf("key order wrong: %s", s)
	}
}

func TestBuildExportRows_ChangedOnlyAndTemplate(t *testing.T) {
	sol := makeTestSolution([]domain.GraphicObject{
		objWithTemplate("obj-a", "Pump", "P1"),
		objWithTemplate("obj-b", "Valve", "V1"),
		objWithTemplate("obj-c", "Pump", "P2"),
	})
	changedIDs := map[string]bool{
		"obj-a": true,
		"obj-b": true,
	}
	req := ExportObjectsRequest{
		Template:    "Pump",
		Fields:      []string{"Id", "Tag"},
		ChangedOnly: true,
	}
	rows := buildExportRows(sol, req, changedIDs)
	if len(rows) != 1 {
		t.Fatalf("changed_only AND Template=Pump should yield 1 row, got %d", len(rows))
	}
	if rows[0]["Id"] != "obj-a" {
		t.Fatalf("expected only changed Pump obj-a, got %v", rows)
	}
}

func TestBuildExportRows_TemplateFilterWhenNotChangedOnly(t *testing.T) {
	sol := makeTestSolution([]domain.GraphicObject{
		objWithTemplate("obj-a", "Pump", "P1"),
		objWithTemplate("obj-b", "Valve", "V1"),
	})
	req := ExportObjectsRequest{
		Template:    "Pump",
		Fields:      []string{"Id"},
		ChangedOnly: false,
	}
	rows := buildExportRows(sol, req, nil)
	if len(rows) != 1 {
		t.Fatalf("expected 1 pump row, got %d", len(rows))
	}
	if rows[0]["Id"] != "obj-a" {
		t.Fatalf("expected obj-a, got %s", rows[0]["Id"])
	}
}

func TestNormalizeTagPresence(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", TagPresenceAll, false},
		{"all", TagPresenceAll, false},
		{"non_empty", TagPresenceNonEmpty, false},
		{"empty", TagPresenceEmpty, false},
		{"bad", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeTagPresence(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("normalizeTagPresence(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeTagPresence(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeTagPresence(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildExportRows_TagPresenceFilter(t *testing.T) {
	sol := makeTestSolution([]domain.GraphicObject{
		objWithTemplate("obj-full", "Pump", "P1"),
		objWithTemplate("obj-empty", "Pump", ""),
		{
			Id:  "obj-space",
			Tag: "   ",
			TechnologyObject: []domain.TechnologyObject{
				{Id: "obj-space", Tag: "   ", Template: "Pump"},
			},
		},
	})

	t.Run("non_empty", func(t *testing.T) {
		req := ExportObjectsRequest{
			Template:    "Pump",
			Fields:      []string{"Id"},
			TagPresence: TagPresenceNonEmpty,
		}
		rows := buildExportRows(sol, req, nil)
		if len(rows) != 1 || rows[0]["Id"] != "obj-full" {
			t.Fatalf("non_empty: got %v", rows)
		}
	})

	t.Run("empty", func(t *testing.T) {
		req := ExportObjectsRequest{
			Template:    "Pump",
			Fields:      []string{"Id"},
			TagPresence: TagPresenceEmpty,
		}
		rows := buildExportRows(sol, req, nil)
		ids := map[string]bool{}
		for _, row := range rows {
			ids[row["Id"]] = true
		}
		if len(rows) != 2 || !ids["obj-empty"] || !ids["obj-space"] {
			t.Fatalf("empty: got %v", rows)
		}
	})

	t.Run("all", func(t *testing.T) {
		req := ExportObjectsRequest{
			Template:    "Pump",
			Fields:      []string{"Id"},
			TagPresence: TagPresenceAll,
		}
		rows := buildExportRows(sol, req, nil)
		if len(rows) != 3 {
			t.Fatalf("all: expected 3 rows, got %d", len(rows))
		}
	})

	t.Run("applies_without_Tag_field", func(t *testing.T) {
		req := ExportObjectsRequest{
			Template:    "Pump",
			Fields:      []string{"Id", "Name"},
			TagPresence: TagPresenceNonEmpty,
		}
		rows := buildExportRows(sol, req, nil)
		if len(rows) != 1 || rows[0]["Id"] != "obj-full" {
			t.Fatalf("filter without Tag column: got %v", rows)
		}
	})
}

func TestSubtractAckedObjectIDs(t *testing.T) {
	ids := map[string]bool{
		"a": true,
		"b": true,
		"c": true,
	}
	acks := []repository.ExportChangeAckRow{
		{EntityType: "graphic_object", EntityXMLID: "b"},
		{EntityType: "control_command", EntityXMLID: "c"},
	}
	subtractAckedObjectIDs(ids, acks)
	if !ids["a"] || ids["b"] || !ids["c"] {
		t.Fatalf("after subtract: %v", ids)
	}
}
