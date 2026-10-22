package service

import (
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func testComplianceObject() domain.GraphicObject {
	return domain.GraphicObject{
		Id:  "obj-1",
		Tag: "OBJ-TAG",
		TechnologyObject: []domain.TechnologyObject{{
			Id:  "obj-1",
			Tag: "TO-TAG",
			TechnologyProperty: []domain.TechnologyProperty{{
				Name:        "Diameter",
				StringValue: "100",
				Tag:         "Zin",
			}},
		}},
	}
}

func TestResolveComplianceField_PropertyValue(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		Parameter:   "Diameter",
		ValueSource: "field",
	}
	val := resolveComplianceField(rule, &obj, to)
	if val != "100" {
		t.Fatalf("expected property value 100, got %q", val)
	}
}

func TestResolveComplianceField_PropertyTag(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		Parameter:   "Diameter",
		ValueSource: "tag",
	}
	val := resolveComplianceField(rule, &obj, to)
	if val != "Zin" {
		t.Fatalf("expected property tag Zin, got %q", val)
	}
}

func TestResolveComplianceField_ObjectTagLegacy(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		Parameter:   "Tag",
		ValueSource: "tag",
	}
	val := resolveComplianceField(rule, &obj, to)
	if val != "TO-TAG" {
		t.Fatalf("expected object tag TO-TAG, got %q", val)
	}
}

func TestResolveComplianceField_TagSourceUsesParameterNotObjectTag(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		Parameter:   "Diameter",
		ValueSource: "tag",
	}
	val := resolveComplianceField(rule, &obj, to)
	if val == "TO-TAG" || val == "OBJ-TAG" {
		t.Fatalf("tag source must use property tag, not object tag; got %q", val)
	}
	if val != "Zin" {
		t.Fatalf("expected property tag Zin, got %q", val)
	}
}

func testComplianceSolutionWithCommand() domain.Solution {
	obj := testComplianceObject()
	obj.TechnologyObject[0].Template = "Valve"
	return domain.Solution{
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
				Target:   "obj-1",
				Tag:      "CMD-TAG",
				Name:     "Open",
				TechnologyProperty: []domain.TechnologyProperty{{
					Name:        "Delay",
					StringValue: "5",
					Tag:         "CMD-DELAY-TAG",
				}},
			}},
		},
	}
}

func TestResolveComplianceValues_CommandField(t *testing.T) {
	sol := testComplianceSolutionWithCommand()
	obj := sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0]
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		ParameterScope: "command",
		EntityTemplate: "ValveOpen",
		Parameter:      "Name",
		ValueSource:    "field",
	}
	val := resolveComplianceValues(rule, sol, &obj, to)
	if val != "Open" {
		t.Fatalf("expected command name Open, got %q", val)
	}
}

func TestResolveComplianceValues_CommandPropertyTag(t *testing.T) {
	sol := testComplianceSolutionWithCommand()
	obj := sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0]
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		ParameterScope: "command",
		EntityTemplate: "ValveOpen",
		Parameter:      "Delay",
		ValueSource:    "tag",
	}
	val := resolveComplianceValues(rule, sol, &obj, to)
	if val != "CMD-DELAY-TAG" {
		t.Fatalf("expected command property tag CMD-DELAY-TAG, got %q", val)
	}
}

func TestResolveComplianceValues_ProtectionField(t *testing.T) {
	obj := testComplianceObject()
	obj.TechnologyObject[0].Template = "Valve"
	sol := domain.Solution{
		Automations: domain.Automations{
			Automation: []domain.Automation{{
				Template: "ValveProtection",
				Target:   "obj-1",
				Tag:      "AUTO-TAG",
			}},
		},
	}
	rule := repository.ComplianceRule{
		ParameterScope: "protection",
		EntityTemplate: "ValveProtection",
		Parameter:      "Tag",
		ValueSource:    "field",
	}
	val := resolveComplianceValues(rule, sol, &obj, &obj.TechnologyObject[0])
	if val != "AUTO-TAG" {
		t.Fatalf("expected protection tag AUTO-TAG, got %q", val)
	}
}

func TestResolveComplianceValues_PropertyField(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		ParameterScope: "property",
		Parameter:      "Diameter",
		ValueSource:    "field",
	}
	val := resolveComplianceValues(rule, domain.Solution{}, &obj, to)
	if val != "100" {
		t.Fatalf("expected property value 100, got %q", val)
	}
}

func TestResolveComplianceValues_PropertyTag(t *testing.T) {
	obj := testComplianceObject()
	to := &obj.TechnologyObject[0]
	rule := repository.ComplianceRule{
		ParameterScope: "property",
		Parameter:      "Diameter",
		ValueSource:    "tag",
	}
	val := resolveComplianceValues(rule, domain.Solution{}, &obj, to)
	if val != "Zin" {
		t.Fatalf("expected property tag Zin, got %q", val)
	}
}

func TestEvaluateObjectComplianceAll_ExactMatchRule(t *testing.T) {
	sol := testComplianceSolutionWithCommand()
	obj := sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0]
	rules := []repository.ComplianceRule{{
		Template:       "Valve",
		ParameterScope: "command",
		EntityTemplate: "ValveOpen",
		Parameter:      "Name",
		ValueSource:    "field",
		TextCondition:  "exact_match",
		ExpectedText:   "Open",
		EmptyAllowed:   false,
		Indication:     []byte(`{"type":"solid","color":"#F85149"}`),
	}}
	rules[0].ID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	indicators := evaluateObjectComplianceAll(sol, obj, rules)
	if len(indicators) != 0 {
		t.Fatalf("expected no violation for exact match, got %d", len(indicators))
	}

	rules[0].ExpectedText = "Close"
	indicators = evaluateObjectComplianceAll(sol, obj, rules)
	if len(indicators) != 1 {
		t.Fatalf("expected 1 violation for exact mismatch, got %d", len(indicators))
	}
}

func TestEvaluateObjectComplianceAll_MustNotExactRule(t *testing.T) {
	sol := testComplianceSolutionWithCommand()
	obj := sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0]
	rules := []repository.ComplianceRule{{
		Template:       "Valve",
		ParameterScope: "command",
		EntityTemplate: "ValveOpen",
		Parameter:      "Name",
		ValueSource:    "field",
		TextCondition:  "must_not_exact",
		ExpectedText:   "0",
		EmptyAllowed:   false,
		Indication:     []byte(`{"type":"solid","color":"#F85149"}`),
	}}
	rules[0].ID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

	sol.ControlCommands.ControlCommand[0].Name = "0"
	indicators := evaluateObjectComplianceAll(sol, obj, rules)
	if len(indicators) != 1 {
		t.Fatalf("expected 1 violation for must_not_exact equal to %q, got %d", "0", len(indicators))
	}

	for _, caseVal := range []string{"10", "01", "0.0"} {
		sol.ControlCommands.ControlCommand[0].Name = caseVal
		indicators = evaluateObjectComplianceAll(sol, obj, rules)
		if len(indicators) != 0 {
			t.Fatalf("expected no violation for must_not_exact val %q vs expected %q, got %d", caseVal, "0", len(indicators))
		}
	}
}

func TestEvaluateObjectComplianceAll_CommandRuleViolation(t *testing.T) {
	sol := testComplianceSolutionWithCommand()
	obj := sol.Projects.Project[0].GraphicScheme[0].GraphicObject[0]
	rules := []repository.ComplianceRule{{
		Template:       "Valve",
		ParameterScope: "command",
		EntityTemplate: "ValveOpen",
		Parameter:      "Name",
		ValueSource:    "field",
		TextCondition:  "must_contain",
		ExpectedText:   "Close",
		EmptyAllowed:   false,
		Indication:     []byte(`{"type":"solid","color":"#F85149"}`),
	}}
	rules[0].ID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	indicators := evaluateObjectComplianceAll(sol, obj, rules)
	if len(indicators) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(indicators))
	}
}
