package service

import (
	"testing"

	"create_sheme/internal/domain"
)

func complianceTargetsSolution() domain.Solution {
	valveA := objWithTemplate("valve-a", "Valve", "V-A")
	valveB := objWithTemplate("valve-b", "Valve", "V-B")
	pump := objWithTemplate("pump-1", "Pump", "P-1")
	return domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{{
				GraphicScheme: []domain.GraphicScheme{{
					GraphicObject: []domain.GraphicObject{valveA, valveB, pump},
				}},
			}},
		},
		ControlCommands: domain.ControlCommands{
			ControlCommand: []domain.ControlCommand{
				{Template: "ValveOpen", Target: "valve-a", Tag: "CMD-OPEN-A"},
				{Template: "ValveClose", Target: "valve-b", Tag: "CMD-CLOSE-B"},
				{Template: "ValveOpen", Target: "valve-b", Tag: "CMD-OPEN-B"},
				{Template: "PumpStart", Target: "pump-1", Tag: "CMD-START"},
				{Template: "", Target: "valve-a", Tag: "CMD-NO-TPL"},
				{Template: "ValveStop", Target: "", Tag: "CMD-NO-TARGET"},
			},
		},
		Automations: domain.Automations{
			Automation: []domain.Automation{
				{Template: "ValveProtection", Target: "valve-a", Tag: "AUTO-A"},
				{Template: "PumpProtection", Target: "pump-1", Tag: "AUTO-P"},
			},
		},
	}
}

func TestListComplianceTargetsFromSolution_Command(t *testing.T) {
	sol := complianceTargetsSolution()
	targets, err := listComplianceTargetsFromSolution(sol, "Valve", "command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 command templates, got %v", targets)
	}
	if targets[0] != "ValveClose" || targets[1] != "ValveOpen" {
		t.Fatalf("unexpected templates: %v", targets)
	}
}

func TestListComplianceTargetsFromSolution_Protection(t *testing.T) {
	sol := complianceTargetsSolution()
	targets, err := listComplianceTargetsFromSolution(sol, "Valve", "protection")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0] != "ValveProtection" {
		t.Fatalf("expected [ValveProtection], got %v", targets)
	}
}

func TestListComplianceTargetsFromSolution_Empty(t *testing.T) {
	sol := complianceTargetsSolution()
	targets, err := listComplianceTargetsFromSolution(sol, "Tank", "command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected empty result, got %v", targets)
	}
}

func TestListComplianceTargetsFromSolution_InvalidScope(t *testing.T) {
	sol := complianceTargetsSolution()
	_, err := listComplianceTargetsFromSolution(sol, "Valve", "object")
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestListComplianceTargetsFromSolution_MatchesTechnologyObjectId(t *testing.T) {
	obj := domain.GraphicObject{
		Id:  "graphic-1",
		Tag: "G-TAG",
		TechnologyObject: []domain.TechnologyObject{{
			Id:       "tech-99",
			Tag:      "TO-TAG",
			Template: "Valve",
		}},
	}
	sol := domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{{
				GraphicScheme: []domain.GraphicScheme{{
					GraphicObject: []domain.GraphicObject{obj},
				}},
			}},
		},
		ControlCommands: domain.ControlCommands{
			ControlCommand: []domain.ControlCommand{{
				Template: "ValveOpen",
				Target:   "tech-99",
			}},
		},
	}
	targets, err := listComplianceTargetsFromSolution(sol, "Valve", "command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0] != "ValveOpen" {
		t.Fatalf("expected [ValveOpen], got %v", targets)
	}
}
