package service

import (
	"encoding/json"
	"strings"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"
)

func TestMarshalLeanSolutionStripsGraphicObjects(t *testing.T) {
	sol := domain.Solution{
		Name: "demo",
		Projects: domain.Projects{
			Project: []domain.Project{{
				Id:   "proj-1",
				Name: "P1",
				GraphicScheme: []domain.GraphicScheme{{
					Id:       "sch-1",
					Name:     "Scheme",
					Template: "T",
					GraphicObject: []domain.GraphicObject{{
						Id:  "obj-1",
						Tag: "TAG1",
					}},
				}},
			}},
		},
	}
	raw, err := marshalLeanSolution(sol)
	if err != nil {
		t.Fatalf("marshalLeanSolution: %v", err)
	}
	var lean domain.Solution
	if err := json.Unmarshal(raw, &lean); err != nil {
		t.Fatalf("unmarshal lean: %v", err)
	}
	if lean.DataLayout != domain.LeanSchemesLayoutV1 {
		t.Fatalf("dataLayout=%q want %q", lean.DataLayout, domain.LeanSchemesLayoutV1)
	}
	if len(lean.Projects.Project) != 1 || len(lean.Projects.Project[0].GraphicScheme) != 1 {
		t.Fatalf("unexpected lean project/scheme shape: %+v", lean.Projects)
	}
	stub := lean.Projects.Project[0].GraphicScheme[0]
	if stub.Id != "sch-1" || stub.Name != "Scheme" || stub.Template != "T" {
		t.Fatalf("stub metadata lost: %+v", stub)
	}
	if len(stub.GraphicObject) != 0 {
		t.Fatalf("GraphicObject not stripped: %d", len(stub.GraphicObject))
	}
	if sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0].Id != "obj-1" {
		t.Fatal("marshalLeanSolution mutated caller solution")
	}
	if !strings.Contains(string(raw), `"dataLayout":"lean-schemes-v1"`) {
		t.Fatalf("marker missing in JSON: %s", raw)
	}
}

func TestHydrateSolutionSchemesMergesByProjectAndXMLID(t *testing.T) {
	sol := domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{{
				Id: "proj-1",
				GraphicScheme: []domain.GraphicScheme{
					domain.SchemeStub(domain.GraphicScheme{Id: "sch-a", Name: "A"}),
					domain.SchemeStub(domain.GraphicScheme{Id: "sch-b", Name: "B"}),
				},
			}},
		},
	}
	payloadA, _ := json.Marshal(domain.GraphicScheme{
		Id:   "sch-a",
		Name: "A-full",
		GraphicObject: []domain.GraphicObject{{
			Id: "obj-a",
		}},
	})
	payloadB, _ := json.Marshal(domain.GraphicScheme{
		Id:   "sch-b",
		Name: "B-full",
		GraphicObject: []domain.GraphicObject{{
			Id: "obj-b",
		}},
	})
	rows := []repository.SchemeRow{
		{XMLID: "sch-b", ProjectXMLID: "proj-1", Payload: payloadB},
		{XMLID: "sch-a", ProjectXMLID: "other", Payload: payloadA},
		{XMLID: "sch-a", ProjectXMLID: "proj-1", Payload: payloadA},
	}
	if err := hydrateSolutionSchemes(&sol, rows); err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	schemes := sol.Projects.Project[0].GraphicScheme
	if schemes[0].Id != "sch-a" || schemes[1].Id != "sch-b" {
		t.Fatalf("stub order broken: %s, %s", schemes[0].Id, schemes[1].Id)
	}
	if len(schemes[0].GraphicObject) != 1 || schemes[0].GraphicObject[0].Id != "obj-a" {
		t.Fatalf("sch-a not hydrated: %+v", schemes[0])
	}
	if len(schemes[1].GraphicObject) != 1 || schemes[1].GraphicObject[0].Id != "obj-b" {
		t.Fatalf("sch-b not hydrated: %+v", schemes[1])
	}
}

func TestCheckSchemePayloadSize(t *testing.T) {
	if schemePayloadHardLimit != 900<<20 {
		t.Fatalf("hard limit=%d want %d", schemePayloadHardLimit, 900<<20)
	}
	if schemePayloadSoftLimit != 240<<20 {
		t.Fatalf("soft limit=%d want %d", schemePayloadSoftLimit, 240<<20)
	}
	if err := checkSchemePayloadSize("ok", make([]byte, 10)); err != nil {
		t.Fatalf("small payload: %v", err)
	}
	if err := checkSchemePayloadByteCount("big", schemePayloadHardLimit+1); err == nil {
		t.Fatal("expected hard limit error")
	}
	if err := checkSchemePayloadByteCount("soft", schemePayloadSoftLimit+1); err != nil {
		t.Fatalf("soft limit should warn only: %v", err)
	}
}
