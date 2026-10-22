package service

import (
	"reflect"
	"testing"

	"create_sheme/internal/domain"

	"github.com/google/uuid"
)

func catalogFixtureSolution() domain.Solution {
	valveA := objWithTemplate("valve-a", "Valve", "V-A")
	valveA.TechnologyObject[0].TechnologyProperty = []domain.TechnologyProperty{
		{Name: "Pressure", Template: "Pressure"},
		{Name: "Tag", Template: "TagProp"},
		{Template: "OnlyTemplate"},
		{Name: "", Template: ""},
	}
	valveB := objWithTemplate("valve-b", "Valve", "V-B")
	valveB.TechnologyObject[0].TechnologyProperty = []domain.TechnologyProperty{
		{Name: "Flow", Template: "Flow"},
		{Name: "Pressure", Template: "Pressure"},
	}
	pump := objWithTemplate("pump-1", "Pump", "P-1")
	pump.TechnologyObject[0].TechnologyProperty = []domain.TechnologyProperty{
		{Name: "RPM", Template: "RPM"},
	}
	noTech := domain.GraphicObject{Id: "bare", Tag: "BARE", TechnologyObject: nil}
	return domain.Solution{
		Projects: domain.Projects{
			Project: []domain.Project{{
				GraphicScheme: []domain.GraphicScheme{{
					GraphicObject: []domain.GraphicObject{valveA, valveB, pump, noTech},
				}},
			}},
		},
		ControlCommands: domain.ControlCommands{
			ControlCommand: []domain.ControlCommand{
				{
					Template: "ValveOpen",
					Target:   "valve-a",
					TechnologyProperty: []domain.TechnologyProperty{
						{Name: "Timeout", Template: "Timeout"},
						{Name: "Name", Template: "NameProp"},
					},
				},
				{
					Template: "ValveClose",
					Target:   "valve-b",
					TechnologyProperty: []domain.TechnologyProperty{
						{Name: "Delay", Template: "Delay"},
					},
				},
				{Template: "ValveOpen", Target: "valve-b"},
				{Template: "", Target: "valve-a"},
				{Template: "Orphan", Target: ""},
				{Template: "PumpStart", Target: "pump-1"},
			},
		},
		Automations: domain.Automations{
			Automation: []domain.Automation{
				{
					Template: "ValveProtection",
					Target:   "valve-a",
					TechnologyProperty: []domain.TechnologyProperty{
						{Name: "Limit", Template: "Limit"},
					},
				},
				{Template: "PumpProtection", Target: "pump-1"},
			},
		},
	}
}

func TestWithBaseParameters(t *testing.T) {
	got := withBaseParameters([]string{"Flow", "Name", "Pressure", ""})
	want := []string{"Description", "ExternalCode", "Id", "Name", "Tag", "Flow", "Pressure"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTemplateParametersUnion(t *testing.T) {
	sol := catalogFixtureSolution()
	rows := buildTemplateParameters(uuid.New(), sol)

	objectValve := lookupTemplateParameters(rows, "object", "Valve")
	wantObject := listTemplateParametersFromSolution(sol, "Valve")
	if !reflect.DeepEqual(objectValve, wantObject) {
		t.Fatalf("object Valve: got %v want %v", objectValve, wantObject)
	}

	propertyValve := lookupTemplateParameters(rows, "property", "Valve")
	wantProperty := listTemplateCharacteristicsFromSolution(sol, "Valve")
	if !reflect.DeepEqual(propertyValve, wantProperty) {
		t.Fatalf("property Valve: got %v want %v", propertyValve, wantProperty)
	}

	commandOpen := lookupTemplateParameters(rows, "command", "ValveOpen")
	wantCommand, err := listComplianceTargetParametersFromSolution(sol, "command", "ValveOpen")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(commandOpen, wantCommand) {
		t.Fatalf("command ValveOpen: got %v want %v", commandOpen, wantCommand)
	}

	protection := lookupTemplateParameters(rows, "protection", "ValveProtection")
	wantProtection, err := listComplianceTargetParametersFromSolution(sol, "protection", "ValveProtection")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(protection, wantProtection) {
		t.Fatalf("protection ValveProtection: got %v want %v", protection, wantProtection)
	}

	if len(lookupTemplateParameters(rows, "object", "Missing")) != 0 {
		t.Fatal("missing template should yield empty lookup")
	}
}

func TestBuildTemplateEntitiesUnion(t *testing.T) {
	sol := catalogFixtureSolution()
	rows := buildTemplateEntities(uuid.New(), sol)

	commands := lookupTemplateEntities(rows, "Valve", "command")
	wantCommands, err := listComplianceTargetsFromSolution(sol, "Valve", "command")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(commands, wantCommands) {
		t.Fatalf("command targets: got %v want %v", commands, wantCommands)
	}

	protections := lookupTemplateEntities(rows, "Valve", "protection")
	wantProtections, err := listComplianceTargetsFromSolution(sol, "Valve", "protection")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(protections, wantProtections) {
		t.Fatalf("protection targets: got %v want %v", protections, wantProtections)
	}

	empty, err := listComplianceTargetsFromSolution(sol, "Tank", "command")
	if err != nil {
		t.Fatal(err)
	}
	got := lookupTemplateEntities(rows, "Tank", "command")
	if len(got) != len(empty) {
		t.Fatalf("empty targets: got %v want %v", got, empty)
	}
}

func TestCatalogEquivalenceAllScopes(t *testing.T) {
	sol := catalogFixtureSolution()
	solutionID := uuid.New()
	paramRows := buildTemplateParameters(solutionID, sol)
	entityRows := buildTemplateEntities(solutionID, sol)

	templates := []string{"Valve", "Pump"}
	for _, tmpl := range templates {
		gotObject := lookupTemplateParameters(paramRows, "object", tmpl)
		wantObject := listTemplateParametersFromSolution(sol, tmpl)
		if !reflect.DeepEqual(gotObject, wantObject) {
			t.Fatalf("object %s: got %v want %v", tmpl, gotObject, wantObject)
		}

		gotProperty := lookupTemplateParameters(paramRows, "property", tmpl)
		wantProperty := listTemplateCharacteristicsFromSolution(sol, tmpl)
		if !reflect.DeepEqual(gotProperty, wantProperty) {
			t.Fatalf("property %s: got %v want %v", tmpl, gotProperty, wantProperty)
		}

		for _, scope := range []string{"command", "protection"} {
			gotTargets := lookupTemplateEntities(entityRows, tmpl, scope)
			wantTargets, err := listComplianceTargetsFromSolution(sol, tmpl, scope)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotTargets, wantTargets) {
				t.Fatalf("%s targets %s: got %v want %v", scope, tmpl, gotTargets, wantTargets)
			}
			for _, entity := range wantTargets {
				gotParams := lookupTemplateParameters(paramRows, scope, entity)
				wantParams, err := listComplianceTargetParametersFromSolution(sol, scope, entity)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(gotParams, wantParams) {
					t.Fatalf("%s params %s: got %v want %v", scope, entity, gotParams, wantParams)
				}
			}
		}
	}
}

func TestCatalogVersionConstant(t *testing.T) {
	if CatalogVersion < 1 {
		t.Fatalf("CatalogVersion must be >= 1, got %d", CatalogVersion)
	}
}
