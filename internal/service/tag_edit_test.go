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

func valveObj(tag string) domain.GraphicObject {
	return domain.GraphicObject{
		Id:  "obj-1",
		Tag: tag,
		TechnologyObject: []domain.TechnologyObject{{
			Id:       "to-1",
			Tag:      "TO-KEEP",
			Template: "Valve",
			TechnologyProperty: []domain.TechnologyProperty{
				{Id: "p1", Name: "Param1", Tag: "PROP-OLD"},
			},
		}},
	}
}

func TestApplyTagRulesToValue_Delete(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Valve", TextCondition: "contains", Pattern: "old", Action: "delete"},
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
		{Template: "Valve", TextCondition: "starts_with", Pattern: "TMP_", Action: "replace", Replacement: "PROD_"},
	}
	obj := valveObj("TMP_123")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "PROD_123" {
		t.Fatalf("expected PROD_123, got %q", got)
	}
	obj = valveObj("OTHER")
	got = applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "OTHER" {
		t.Fatalf("expected OTHER, got %q", got)
	}
}

func TestApplyTagRulesToValue_ContainsReplaceAll(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Valve", TextCondition: "contains", Pattern: "A", Action: "replace", Replacement: "B"},
	}
	obj := valveObj("XAYAZ")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "XBYBZ" {
		t.Fatalf("expected XBYBZ, got %q", got)
	}
}

func TestApplyTagRulesToValue_Pipeline(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Valve", TextCondition: "contains", Pattern: "A", Action: "replace", Replacement: "B"},
		{Template: "Valve", TextCondition: "contains", Pattern: "B", Action: "delete"},
	}
	obj := valveObj("XA")
	got := applyTagRulesToValue(rules, obj.Tag, obj, SolutionMetadata{})
	if got != "" {
		t.Fatalf("expected pipeline delete, got %q", got)
	}
}

func TestApplyTagEditToObject_OnlyMainTag(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Valve", TextCondition: "contains", Pattern: "OLD", Action: "replace", Replacement: "NEW"},
	}
	obj := valveObj("MAIN-OLD")
	updated, changed := applyTagEditToObject(obj, rules, SolutionMetadata{})
	if !changed {
		t.Fatal("expected changed=true")
	}
	if updated.Tag != "MAIN-NEW" {
		t.Fatalf("expected MAIN-NEW, got %q", updated.Tag)
	}
	if updated.TechnologyObject[0].Tag != "TO-KEEP" {
		t.Fatalf("nested TO tag must stay, got %q", updated.TechnologyObject[0].Tag)
	}
	if updated.TechnologyObject[0].TechnologyProperty[0].Tag != "PROP-OLD" {
		t.Fatalf("property tag must stay, got %q", updated.TechnologyObject[0].TechnologyProperty[0].Tag)
	}
}

func TestApplyTagEditToObject_SkipsOtherTemplate(t *testing.T) {
	rules := []repository.TagEditRule{
		{Template: "Tank", TextCondition: "contains", Pattern: "OLD", Action: "replace", Replacement: "NEW"},
	}
	obj := valveObj("MAIN-OLD")
	updated, changed := applyTagEditToObject(obj, rules, SolutionMetadata{})
	if changed {
		t.Fatal("expected no change for other template")
	}
	if updated.Tag != "MAIN-OLD" {
		t.Fatalf("tag must stay, got %q", updated.Tag)
	}
}

func TestApplyTagEditToObject_EmptyMainWithNestedTagFills(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:        "Valve",
			TextCondition:   "contains",
			Pattern:         ".",
			Action:          "replace",
			Replacement:     "*",
			EmptyAllowed:    false,
			CheckProperties: true,
		},
	}
	obj := valveObj("")
	updated, changed := applyTagEditToObject(obj, rules, SolutionMetadata{})
	if !changed {
		t.Fatal("expected fill of main tag")
	}
	if updated.Tag != "*" {
		t.Fatalf("expected *, got %q", updated.Tag)
	}
	if updated.TechnologyObject[0].TechnologyProperty[0].Tag != "PROP-OLD" {
		t.Fatal("nested tag must not be written")
	}
}

func TestApplyTagEditToObject_EmptyMainAndNestedNoOp(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:        "Valve",
			TextCondition:   "contains",
			Pattern:         ".",
			Action:          "replace",
			Replacement:     "*",
			EmptyAllowed:    false,
			CheckProperties: true,
		},
	}
	obj := valveObj("")
	obj.TechnologyObject[0].TechnologyProperty[0].Tag = ""
	updated, changed := applyTagEditToObject(obj, rules, SolutionMetadata{})
	if changed {
		t.Fatal("expected no-op when nested tags empty")
	}
	if updated.Tag != "" {
		t.Fatalf("expected empty, got %q", updated.Tag)
	}
}

func TestApplyTagEditToObject_EmptyAllowedSkipsEmpty(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:        "Valve",
			TextCondition:   "contains",
			Pattern:         ".",
			Action:          "replace",
			Replacement:     "*",
			EmptyAllowed:    true,
			CheckProperties: true,
		},
	}
	obj := valveObj("")
	_, changed := applyTagEditToObject(obj, rules, SolutionMetadata{})
	if changed {
		t.Fatal("empty allowed must skip empty main tag")
	}
}

func TestApplyTagEditToObject_EmptyMainWithCommandTag(t *testing.T) {
	rules := []repository.TagEditRule{
		{
			Template:      "Valve",
			TextCondition: "contains",
			Pattern:       ".",
			Action:        "replace",
			Replacement:   "#",
			EmptyAllowed:  false,
			CheckCommands: true,
		},
	}
	obj := valveObj("")
	obj.TechnologyObject[0].TechnologyProperty[0].Tag = ""
	meta := SolutionMetadata{
		Commands: []domain.ControlCommand{{Id: "c1", Tag: "CMD", Target: "obj-1"}},
	}
	updated, changed := applyTagEditToObject(obj, rules, meta)
	if !changed {
		t.Fatal("expected fill from command tag")
	}
	if updated.Tag != "#" {
		t.Fatalf("expected #, got %q", updated.Tag)
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
