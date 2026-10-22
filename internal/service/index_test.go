package service

import (
	"encoding/json"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func TestBuildTypeSchemasFromPassport(t *testing.T) {
	sol := domain.Solution{
		TechnologyPassports: domain.TechnologyPassports{
			TechnologyPassport: []domain.TechnologyPassport{
				{Template: "Valve", Name: "Valve Type"},
			},
		},
	}
	solutionID := uuid.New()
	rows := buildTypeSchemas(solutionID, sol)
	if len(rows) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(rows))
	}
	if rows[0].Source != "passport" {
		t.Fatalf("expected passport source, got %s", rows[0].Source)
	}
	if rows[0].Template != "Valve" {
		t.Fatalf("expected Valve template, got %s", rows[0].Template)
	}
}

func TestBuildSolutionObjectsTargetKeys(t *testing.T) {
	sol := domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{
				{
					GraphicScheme: []domain.GraphicScheme{
						{
							Id: "sch1",
							GraphicObject: []domain.GraphicObject{
								{
									Id:  "obj1",
									Tag: "TAG1",
									TechnologyObject: []domain.TechnologyObject{
										{Id: "tech1", Template: "Valve", Tag: "TECH_TAG"},
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
		t.Fatalf("expected 1 object, got %d", len(rows))
	}
	keys := rows[0].TargetKeys
	if len(keys) < 3 {
		t.Fatalf("expected at least 3 target keys, got %v", keys)
	}
}

func TestBuildCommandTargets(t *testing.T) {
	sol := domain.Solution{
		ControlCommands: domain.ControlCommands{
			ControlCommand: []domain.ControlCommand{
				{Id: "cmd1", Target: "TAG1"},
				{Id: "cmd2", Target: ""},
			},
		},
		Automations: domain.Automations{
			Automation: []domain.Automation{
				{Id: "auto1", Target: "TAG1"},
			},
		},
	}
	rows := buildCommandTargets(uuid.New(), sol)
	if len(rows) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(rows))
	}
}

func TestBuildObjectDetailsFromMetadataAndTargets(t *testing.T) {
	meta := SolutionMetadata{
		Commands: []domain.ControlCommand{
			{Id: "cmd1", Target: "A"},
			{Id: "cmd2", Target: "B"},
		},
		Automations: []domain.Automation{
			{Id: "auto1", Target: "A"},
		},
	}
	obj := domain.GraphicObject{Id: "obj1", Tag: "A"}
	refs := []repository.CommandTargetRef{
		{EntityType: "control_command", EntityXMLID: "cmd1"},
		{EntityType: "automation", EntityXMLID: "auto1"},
	}
	details := buildObjectDetailsFromMetadataAndTargets(meta, obj, refs)
	if len(details.Commands) != 1 || details.Commands[0].Id != "cmd1" {
		t.Fatalf("unexpected commands: %+v", details.Commands)
	}
	if len(details.Automations) != 1 || details.Automations[0].Id != "auto1" {
		t.Fatalf("unexpected automations: %+v", details.Automations)
	}
}

func TestBuildTypeSchemasInferred(t *testing.T) {
	sol := domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{
				{
					GraphicScheme: []domain.GraphicScheme{
						{
							GraphicObject: []domain.GraphicObject{
								{
									TechnologyObject: []domain.TechnologyObject{
										{Template: "Pipe", Name: "Pipe Instance"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	rows := buildTypeSchemas(uuid.New(), sol)
	if len(rows) != 1 {
		t.Fatalf("expected 1 inferred schema, got %d", len(rows))
	}
	if rows[0].Source != "inferred" {
		t.Fatalf("expected inferred, got %s", rows[0].Source)
	}
	var to domain.TechnologyObject
	if err := json.Unmarshal(rows[0].Schema, &to); err != nil {
		t.Fatal(err)
	}
	if to.Template != "Pipe" {
		t.Fatalf("expected Pipe, got %s", to.Template)
	}
}
