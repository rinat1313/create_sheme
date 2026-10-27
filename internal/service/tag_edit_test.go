package service

import (
	"encoding/json"
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"
)

func tagAction(parameter, action, replacement, mode string) repository.TagEditAction {
	return repository.TagEditAction{
		ParameterScope: "object",
		Parameter:      parameter,
		ValueSource:    "field",
		Action:         action,
		Replacement:    replacement,
		ReplaceMode:    mode,
	}
}

func valveRule(cond, pattern string, actions ...repository.TagEditAction) repository.TagEditRule {
	return repository.TagEditRule{
		Template:       "Valve",
		ParameterScope: "object",
		Parameter:      "Tag",
		ValueSource:    "field",
		TextCondition:  cond,
		Pattern:        pattern,
		Actions:        actions,
	}
}

func TestMatchTagCondition(t *testing.T) {
	cases := []struct {
		cond    string
		pattern string
		tag     string
		want    bool
	}{
		{"contains", "abc", "xabcy", true},
		{"contains", "abc", "xyz", false},
		{"starts_with", "pre", "prefix", true},
		{"starts_with", "pre", "xpre", false},
		{"ends_with", "suf", "xsuf", true},
		{"ends_with", "suf", "suffixx", false},
		{"contains", "", "anything", false},
		{"empty", "", "", true},
		{"empty", "", "  ", true},
		{"empty", "", "x", false},
	}
	for _, tc := range cases {
		got := matchTagCondition(tc.cond, tc.pattern, tc.tag)
		if got != tc.want {
			t.Fatalf("matchTagCondition(%q, %q, %q) = %v, want %v", tc.cond, tc.pattern, tc.tag, got, tc.want)
		}
	}
}

func valveObj(tag string) domain.GraphicObject {
	return domain.GraphicObject{
		Id:  "obj-1",
		Tag: tag,
		TechnologyObject: []domain.TechnologyObject{{
			Id:       "to-1",
			Tag:      tag,
			Template: "Valve",
			TechnologyProperty: []domain.TechnologyProperty{
				{Id: "p1", Name: "Param1", Tag: "PROP-OLD"},
			},
		}},
	}
}

func TestApplyTagRulesToValue_Delete(t *testing.T) {
	rules := []repository.TagEditRule{
		valveRule("contains", "old", tagAction("Tag", "delete", "", "found")),
	}
	obj := valveObj("prefix-old-suffix")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	obj = valveObj("unchanged")
	got = applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "unchanged" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestApplyTagRulesToValue_ReplaceFragment(t *testing.T) {
	rules := []repository.TagEditRule{
		valveRule("starts_with", "TMP_", tagAction("Tag", "replace", "PROD_", "found")),
	}
	obj := valveObj("TMP_123")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "PROD_123" {
		t.Fatalf("expected PROD_123, got %q", got)
	}
}

func TestApplyTagRulesToValue_ReplaceModes(t *testing.T) {
	obj := valveObj("A-X-A")
	whole := applyTagRulesToValue([]repository.TagEditRule{
		valveRule("contains", "A", tagAction("Tag", "replace", "Z", "whole")),
	}, obj.Tag, obj, SolutionMetadata{})
	if whole != "Z" {
		t.Fatalf("whole: got %q", whole)
	}
	first := applyTagRulesToValue([]repository.TagEditRule{
		valveRule("contains", "A", tagAction("Tag", "replace", "Z", "first")),
	}, obj.Tag, obj, SolutionMetadata{})
	if first != "Z-X-A" {
		t.Fatalf("first: got %q", first)
	}
	last := applyTagRulesToValue([]repository.TagEditRule{
		valveRule("contains", "A", tagAction("Tag", "replace", "Z", "last")),
	}, obj.Tag, obj, SolutionMetadata{})
	if last != "A-X-Z" {
		t.Fatalf("last: got %q", last)
	}
	all := applyTagRulesToValue([]repository.TagEditRule{
		valveRule("contains", "A", tagAction("Tag", "replace", "B", "all")),
	}, obj.Tag, obj, SolutionMetadata{})
	if all != "B-X-B" {
		t.Fatalf("all: got %q", all)
	}
}

func TestApplyTagRulesToValue_Pipeline(t *testing.T) {
	rules := []repository.TagEditRule{
		valveRule("contains", "A", tagAction("Tag", "replace", "B", "all")),
		valveRule("contains", "B", tagAction("Tag", "delete", "", "found")),
	}
	obj := valveObj("XA")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "" {
		t.Fatalf("expected pipeline delete, got %q", got)
	}
}

func TestApplyTagEditToObject_SkipsOtherTemplate(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Tank", ParameterScope: "object", Parameter: "Tag", TextCondition: "contains", Pattern: "OLD", Actions: []repository.TagEditAction{tagAction("Tag", "replace", "NEW", "found")}},
	}
	obj := valveObj("MAIN-OLD")
	meta := SolutionMetadata{}
	updated, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if changed {
		t.Fatal("expected no change for other template")
	}
	if updated.Tag != "MAIN-OLD" {
		t.Fatalf("tag must stay, got %q", updated.Tag)
	}
}

func TestApplyTagEditToObject_EmptyConditionWritesReplacement(t *testing.T) {
	rules := []repository.TagEditRule{
		valveRule("empty", "", tagAction("Tag", "replace", "*", "whole")),
	}
	obj := valveObj("")
	obj.TechnologyObject[0].TechnologyProperty[0].Tag = ""
	meta := SolutionMetadata{}
	updated, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if !changed {
		t.Fatal("expected change for empty tag")
	}
	if updated.Tag != "*" {
		t.Fatalf("expected *, got %q", updated.Tag)
	}
}

func TestApplyTagEditToObject_NestedANDRequiresNonEmpty(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:        "Valve",
			ParameterScope:  "object",
			Parameter:       "Tag",
			TextCondition:   "empty",
			CheckProperties: true,
			Actions:         []repository.TagEditAction{tagAction("Tag", "replace", "*", "whole")},
		},
	}
	obj := valveObj("")
	obj.TechnologyObject[0].TechnologyProperty[0].Tag = ""
	meta := SolutionMetadata{}
	_, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if changed {
		t.Fatal("empty nested tags must keep object out of selection")
	}
	obj.TechnologyObject[0].TechnologyProperty[0].Tag = "PROP"
	updated, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if !changed || updated.Tag != "*" {
		t.Fatalf("non-empty nested tag must select object, tag=%q changed=%v", updated.Tag, changed)
	}
}

func TestApplyTagEditToObject_NoLegacyFillWithoutEmptyCondition(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:        "Valve",
			ParameterScope:  "object",
			Parameter:       "Tag",
			TextCondition:   "contains",
			Pattern:         ".",
			CheckProperties: true,
			Actions:         []repository.TagEditAction{tagAction("Tag", "replace", "*", "whole")},
		},
	}
	obj := valveObj("")
	meta := SolutionMetadata{}
	_, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if changed {
		t.Fatal("legacy fill must be gone")
	}
}

func TestApplyTagEditToObject_WritesPropertyTag(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:       "Valve",
			ParameterScope: "object",
			Parameter:      "Tag",
			TextCondition:  "contains",
			Pattern:        "MAIN",
			Actions: []repository.TagEditAction{{
				ParameterScope: "property",
				Parameter:      "Param1",
				ValueSource:    "tag",
				Action:         "replace",
				Replacement:    "NEW",
				ReplaceMode:    "whole",
			}},
		},
	}
	obj := valveObj("MAIN")
	meta := SolutionMetadata{}
	updated, changed, _ := applyTagEditToObject(obj, rules, &meta)
	if !changed {
		t.Fatal("expected property change")
	}
	if updated.TechnologyObject[0].TechnologyProperty[0].Tag != "NEW" {
		t.Fatalf("property tag: %q", updated.TechnologyObject[0].TechnologyProperty[0].Tag)
	}
}

func TestApplyTagEditToObject_WritesCommandTag(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:       "Valve",
			ParameterScope: "object",
			Parameter:      "Tag",
			TextCondition:  "empty",
			Actions: []repository.TagEditAction{{
				ParameterScope: "command",
				EntityTemplate: "Open",
				Parameter:      "Tag",
				ValueSource:    "field",
				Action:         "replace",
				Replacement:    "CMD-NEW",
				ReplaceMode:    "whole",
			}},
		},
	}
	obj := valveObj("")
	meta := SolutionMetadata{
		Commands: []domain.ControlCommand{{Id: "c1", Template: "Open", Tag: "CMD", Target: "obj-1"}},
	}
	_, objectChanged, metaChanged := applyTagEditToObject(obj, rules, &meta)
	if objectChanged {
		t.Fatal("object must stay")
	}
	if !metaChanged || meta.Commands[0].Tag != "CMD-NEW" {
		t.Fatalf("command tag: %q meta=%v", meta.Commands[0].Tag, metaChanged)
	}
}

func TestTagEditMarksSolutionDirty(t *testing.T) {
	if tagEditMarksSolutionDirty(0) {
		t.Fatal("expected dirty=false when changed=0")
	}
	if !tagEditMarksSolutionDirty(1) {
		t.Fatal("expected dirty=true when changed=1")
	}
}

func TestBuildTagEditRule_EmptyAllowsBlankPattern(t *testing.T) {
	rule, err := buildTagEditRule([16]byte{}, [16]byte{}, TagEditRuleInput{
		Template:      "Valve",
		TextCondition: "empty",
		Actions:       []TagEditActionInput{{Parameter: "Tag", Action: "delete"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Pattern != "" {
		t.Fatalf("pattern must be empty, got %q", rule.Pattern)
	}
}

func TestTagEditExportRoundTripShape(t *testing.T) {
	raw, _ := json.MarshalIndent(tagEditRulesExport{
		Version: 1,
		Kind:    "tag-edit-rules",
		Rules: []tagEditRuleExportItem{{
			Template:      "Valve",
			TextCondition: "empty",
			Actions:       []repository.TagEditAction{tagAction("Tag", "delete", "", "found")},
		}},
	}, "", "  ")
	var parsed tagEditRulesExport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != "tag-edit-rules" || len(parsed.Rules) != 1 {
		t.Fatalf("unexpected export: %+v", parsed)
	}
}
