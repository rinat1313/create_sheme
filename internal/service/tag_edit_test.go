package service

import (
	"testing"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"
)

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
	}
	for _, tc := range cases {
		got := matchTagCondition(tc.cond, tc.pattern, tc.tag)
		if got != tc.want {
			t.Fatalf("matchTagCondition(%q, %q, %q) = %v, want %v", tc.cond, tc.pattern, tc.tag, got, tc.want)
		}
	}
}

func TestApplyTagRulesToValue_Delete(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "contains", Pattern: "old", Action: "delete"},
	}
	got := applyTagRulesToValue(rules, "prefix-old-suffix")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	got = applyTagRulesToValue(rules, "unchanged")
	if got != "unchanged" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestApplyTagRulesToValue_Replace(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "starts_with", Pattern: "TMP_", Action: "replace", Replacement: "PROD_"},
	}
	got := applyTagRulesToValue(rules, "TMP_123")
	if got != "PROD_" {
		t.Fatalf("expected PROD_, got %q", got)
	}
	got = applyTagRulesToValue(rules, "OTHER")
	if got != "OTHER" {
		t.Fatalf("expected OTHER, got %q", got)
	}
}

func TestApplyTagRulesToValue_Pipeline(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "contains", Pattern: "A", Action: "replace", Replacement: "B"},
		{TextCondition: "contains", Pattern: "B", Action: "delete"},
	}
	got := applyTagRulesToValue(rules, "XA")
	if got != "" {
		t.Fatalf("expected pipeline delete, got %q", got)
	}
}

func TestApplyTagEditToObject_Properties(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "contains", Pattern: "OLD", Action: "replace", Replacement: "NEW"},
	}
	obj := domain.GraphicObject{
		Id:  "obj-1",
		Tag: "MAIN-OLD",
		TechnologyObject: []domain.TechnologyObject{{
			Id:  "to-1",
			Tag: "TO-OLD",
			TechnologyProperty: []domain.TechnologyProperty{
				{Id: "p1", Name: "Param1", Tag: "PROP-OLD"},
				{Id: "p2", Name: "Param2", Tag: "KEEP"},
			},
		}},
	}
	updated, changed := applyTagEditToObject(obj, rules)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if updated.Tag != "NEW" {
		t.Fatalf("expected NEW, got %q", updated.Tag)
	}
	if updated.TechnologyObject[0].Tag != "NEW" {
		t.Fatalf("expected NEW, got %q", updated.TechnologyObject[0].Tag)
	}
	if updated.TechnologyObject[0].TechnologyProperty[0].Tag != "NEW" {
		t.Fatalf("expected NEW, got %q", updated.TechnologyObject[0].TechnologyProperty[0].Tag)
	}
	if updated.TechnologyObject[0].TechnologyProperty[1].Tag != "KEEP" {
		t.Fatalf("expected KEEP, got %q", updated.TechnologyObject[0].TechnologyProperty[1].Tag)
	}
}

func TestApplyTagEditToControlCommand(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "starts_with", Pattern: "TMP_", Action: "delete"},
	}
	cmd := domain.ControlCommand{
		Id:     "cmd-1",
		Tag:    "TMP_CMD",
		Target: "to-1",
		TechnologyProperty: []domain.TechnologyProperty{
			{Id: "cp1", Tag: "TMP_PROP"},
			{Id: "cp2", Tag: "PROD_PROP"},
		},
	}
	updated, changed := applyTagEditToControlCommand(cmd, rules)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if updated.Tag != "" {
		t.Fatalf("expected empty command tag, got %q", updated.Tag)
	}
	if updated.TechnologyProperty[0].Tag != "" {
		t.Fatalf("expected empty property tag, got %q", updated.TechnologyProperty[0].Tag)
	}
	if updated.TechnologyProperty[1].Tag != "PROD_PROP" {
		t.Fatalf("expected PROD_PROP unchanged, got %q", updated.TechnologyProperty[1].Tag)
	}
}

func TestApplyTagEditToAutomation(t *testing.T) {
	rules := []repository.TagEditRule{
		{TextCondition: "contains", Pattern: "BAD", Action: "replace", Replacement: "OK"},
	}
	auto := domain.Automation{
		Id:     "auto-1",
		Tag:    "AUTO-BAD",
		Target: "obj-1",
		TechnologyProperty: []domain.TechnologyProperty{
			{Id: "ap1", Tag: "PROP-BAD"},
		},
	}
	updated, changed := applyTagEditToAutomation(auto, rules)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if updated.Tag != "OK" {
		t.Fatalf("expected OK, got %q", updated.Tag)
	}
	if updated.TechnologyProperty[0].Tag != "OK" {
		t.Fatalf("expected OK, got %q", updated.TechnologyProperty[0].Tag)
	}
}

func TestTagEditMarksSolutionDirty(t *testing.T) {
	if tagEditMarksSolutionDirty(0) {
		t.Fatal("expected dirty=false when changed=0")
	}
	if !tagEditMarksSolutionDirty(1) {
		t.Fatal("expected dirty=true when changed=1")
	}
	if !tagEditMarksSolutionDirty(42) {
		t.Fatal("expected dirty=true when changed>0")
	}
}
