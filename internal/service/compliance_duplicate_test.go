package service

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func makeDuplicateScanSolution(objects []domain.GraphicObject, commands []domain.ControlCommand, automations []domain.Automation) domain.Solution {
	sol := makeTestSolution(objects)
	sol.ControlCommands.ControlCommand = commands
	sol.Automations.Automation = automations
	return sol
}

func TestBuildDuplicateTagIndex_EmptyTagsIgnored(t *testing.T) {
	sol := makeDuplicateScanSolution([]domain.GraphicObject{
		{
			Id: "obj-1",
			TechnologyObject: []domain.TechnologyObject{{
				Id: "obj-1", Tag: "A",
				TechnologyProperty: []domain.TechnologyProperty{
					{Name: "P1", Tag: ""},
					{Name: "P2", Tag: "   "},
				},
			}},
		},
	}, nil, nil)

	var all []TagOccurrence
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				all = append(all, collectTagOccurrencesForObject(sol, scheme.Id, scheme.Name, obj, nil, nil)...)
			}
		}
	}

	violations, duplicateTags := BuildDuplicateTagIndex(all)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
	if len(duplicateTags) != 0 {
		t.Fatalf("expected no duplicate tags, got %v", duplicateTags)
	}
}

func TestBuildDuplicateTagIndex_CrossObjectDuplicate(t *testing.T) {
	sol := makeDuplicateScanSolution([]domain.GraphicObject{
		{Id: "obj-1", TechnologyObject: []domain.TechnologyObject{{Id: "obj-1", Tag: "SHARED"}}},
		{Id: "obj-2", TechnologyObject: []domain.TechnologyObject{{Id: "obj-2", Tag: "SHARED"}}},
	}, nil, nil)

	var all []TagOccurrence
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				all = append(all, collectTagOccurrencesForObject(sol, scheme.Id, scheme.Name, obj, nil, nil)...)
			}
		}
	}

	violations, duplicateTags := BuildDuplicateTagIndex(all)
	if len(duplicateTags) != 1 || duplicateTags[0] != "SHARED" {
		t.Fatalf("expected SHARED duplicate, got %v", duplicateTags)
	}
	if !violations["obj-1"] || !violations["obj-2"] {
		t.Fatalf("expected both objects violated, got %v", violations)
	}
}

func TestBuildDuplicateTableRows_SortedByTag(t *testing.T) {
	rows := []TagOccurrence{
		{Tag: "B-TAG", ObjectID: "obj-2", Scope: "object"},
		{Tag: "A-TAG", ObjectID: "obj-1", Scope: "object"},
		{Tag: "A-TAG", ObjectID: "obj-3", Scope: "property"},
		{Tag: "UNIQUE", ObjectID: "obj-4", Scope: "object"},
	}
	result := BuildDuplicateTableRows(rows, []string{"A-TAG", "B-TAG"})
	if len(result) != 3 {
		t.Fatalf("expected 3 duplicate rows, got %d", len(result))
	}
	if result[0].Tag != "A-TAG" || result[2].Tag != "B-TAG" {
		t.Fatalf("expected sort by tag, got %v", result)
	}
}

func TestCollectTagOccurrences_CommandViaMatchesTarget(t *testing.T) {
	obj := domain.GraphicObject{
		Id:  "obj-1",
		Tag: "OBJ-TAG",
		TechnologyObject: []domain.TechnologyObject{{
			Id: "to-1", Tag: "TO-TAG",
		}},
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, []domain.ControlCommand{
		{Tag: "CMD-TAG", Target: "to-1", Template: "Open"},
		{Tag: "OTHER", Target: "other-obj", Template: "Close"},
		{
			Tag: "CMD-PROP", Target: "obj-1", Template: "Set",
			TechnologyProperty: []domain.TechnologyProperty{{Name: "Param", Tag: "PROP-TAG"}},
		},
	}, nil)

	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, nil, nil)
	tags := map[string]int{}
	for _, occ := range occurrences {
		tags[occ.Tag]++
	}
	if tags["CMD-TAG"] != 1 {
		t.Fatalf("expected CMD-TAG from command targeting to-1, got tags %v", tags)
	}
	if tags["OTHER"] != 0 {
		t.Fatalf("command targeting other object should be excluded, got tags %v", tags)
	}
	if tags["CMD-PROP"] != 1 || tags["PROP-TAG"] != 1 {
		t.Fatalf("expected command and property tags for obj-1, got %v", tags)
	}
}

func TestCollectAllTagOccurrencesSequential_EquivalentToParallel(t *testing.T) {
	sol := makeDuplicateScanSolution([]domain.GraphicObject{
		{
			Id: "obj-1",
			TechnologyObject: []domain.TechnologyObject{{
				Id: "to-1", Tag: "SHARED",
				TechnologyProperty: []domain.TechnologyProperty{
					{Name: "P1", Tag: "PROP-A"},
				},
			}},
		},
		{
			Id: "obj-2",
			TechnologyObject: []domain.TechnologyObject{{
				Id: "to-2", Tag: "SHARED",
			}},
		},
		{
			Id: "obj-3",
			TechnologyObject: []domain.TechnologyObject{{
				Id: "to-3", Tag: "UNIQUE",
			}},
		},
	}, []domain.ControlCommand{
		{Tag: "CMD-TAG", Target: "to-1", Template: "Open"},
	}, []domain.Automation{
		{Tag: "AUTO-TAG", Target: "obj-3", Template: "Protect"},
	})

	var refs []tagEditObjectRef
	schemeNames := map[string]string{}
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			schemeNames[scheme.Id] = scheme.Name
			for _, obj := range scheme.GraphicObject {
				refs = append(refs, tagEditObjectRef{schemeXMLID: scheme.Id, obj: obj})
			}
		}
	}

	sequential := collectAllTagOccurrencesSequential(sol, refs, schemeNames, nil, nil, nil)
	parallel := collectAllTagOccurrencesParallel(sol, refs, schemeNames, nil, nil, nil)

	seqViolations, seqDuplicateTags := BuildDuplicateTagIndex(sequential)
	parViolations, parDuplicateTags := BuildDuplicateTagIndex(parallel)

	if len(sequential) != len(parallel) {
		t.Fatalf("occurrence count mismatch: sequential=%d parallel=%d", len(sequential), len(parallel))
	}
	if !reflect.DeepEqual(seqViolations, parViolations) {
		t.Fatalf("violations mismatch: sequential=%v parallel=%v", seqViolations, parViolations)
	}
	sort.Strings(seqDuplicateTags)
	sort.Strings(parDuplicateTags)
	if !reflect.DeepEqual(seqDuplicateTags, parDuplicateTags) {
		t.Fatalf("duplicate tags mismatch: sequential=%v parallel=%v", seqDuplicateTags, parDuplicateTags)
	}
}

func TestForcedScanThrottle_SingleWorker(t *testing.T) {
	workers, batchSize, sleep := forcedScanThrottleConfig()
	if workers != 1 {
		t.Fatalf("expected workers=1, got %d", workers)
	}
	if batchSize < 20 || batchSize > 40 {
		t.Fatalf("expected batchSize 20-40, got %d", batchSize)
	}
	if sleep < 50*time.Millisecond || sleep > 100*time.Millisecond {
		t.Fatalf("expected sleep 50-100ms, got %v", sleep)
	}
}

func TestTagCountEngine_ForcedPathBuildsDuplicateTags(t *testing.T) {
	engine := newTagCountEngine()
	engine.adjustObject("obj-1", []TagOccurrence{
		{Tag: "DUP", ObjectID: "obj-1", SchemeXMLID: "sch-1", SourceLabel: "Tag"},
	})
	engine.adjustObject("obj-2", []TagOccurrence{
		{Tag: "DUP", ObjectID: "obj-2", SchemeXMLID: "sch-1", SourceLabel: "Tag"},
	})
	tags := engine.duplicateTags()
	if len(tags) != 1 || tags[0] != "DUP" {
		t.Fatalf("expected DUP, got %v", tags)
	}
	if engine.countFor("DUP") != 2 {
		t.Fatalf("expected count 2, got %d", engine.countFor("DUP"))
	}
}

func TestCollectTagOccurrences_ExcludesProtectionScope(t *testing.T) {
	obj := domain.GraphicObject{
		Id:  "obj-1",
		Tag: "OBJ-TAG",
		TechnologyObject: []domain.TechnologyObject{{
			Id: "to-1", Tag: "TO-TAG",
		}},
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, nil, []domain.Automation{
		{Tag: "DUP-TAG", Target: "obj-1", Template: "Protect"},
		{Tag: "DUP-TAG", Target: "obj-2", Template: "Protect2"},
	})

	excluded := map[string]bool{"protection": true}
	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, excluded, nil)
	for _, occ := range occurrences {
		if occ.Scope == "protection" {
			t.Fatalf("protection scope should be excluded, got %+v", occ)
		}
	}
	if len(occurrences) == 0 {
		t.Fatal("expected object tags to remain")
	}
}

func TestBuildDuplicateTagIndex_ProtectionExcludedRemovesDuplicate(t *testing.T) {
	obj1 := domain.GraphicObject{Id: "obj-1", TechnologyObject: []domain.TechnologyObject{{Id: "to-1", Tag: "MAIN"}}}
	obj2 := domain.GraphicObject{Id: "obj-2", TechnologyObject: []domain.TechnologyObject{{Id: "to-2", Tag: "MAIN"}}}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj1, obj2}, nil, []domain.Automation{
		{Tag: "DUP-TAG", Target: "obj-1", Template: "Protect"},
		{Tag: "DUP-TAG", Target: "obj-2", Template: "Protect2"},
	})

	excluded := map[string]bool{"protection": true}
	var all []TagOccurrence
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				all = append(all, collectTagOccurrencesForObject(sol, scheme.Id, scheme.Name, obj, excluded, nil)...)
			}
		}
	}

	violations, duplicateTags := BuildDuplicateTagIndex(all)
	if len(duplicateTags) != 1 || duplicateTags[0] != "MAIN" {
		t.Fatalf("expected only MAIN duplicate, got %v", duplicateTags)
	}
	if !violations["obj-1"] || !violations["obj-2"] {
		t.Fatalf("expected MAIN duplicate violations, got %v", violations)
	}
}

func TestParseDuplicateExcludedScopes_FromIndication(t *testing.T) {
	indication, err := json.Marshal(map[string]interface{}{
		"type":            "solid",
		"color":           "#F85149",
		"excluded_scopes": []string{"command", "protection"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rule := repository.ComplianceRule{Indication: indication}
	excluded := parseDuplicateExcludedScopes(rule)
	if !excluded["command"] || !excluded["protection"] || excluded["object"] {
		t.Fatalf("unexpected excluded scopes: %v", excluded)
	}
}

func TestParseDuplicateExcludedTagPatterns_FromIndication(t *testing.T) {
	indication, err := json.Marshal(map[string]interface{}{
		"excluded_tag_patterns": []map[string]string{
			{"mode": "contains_char", "value": "_"},
			{"mode": "contains_text", "value": "TMP"},
			{"mode": "contains_char", "value": "ab"},
			{"mode": "contains_char", "value": " "},
			{"mode": "contains_text", "value": "   "},
			{"mode": "equals", "value": "X"},
			{"mode": "contains_char", "value": ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rule := repository.ComplianceRule{Indication: indication}
	patterns := parseDuplicateExcludedTagPatterns(rule)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 valid patterns, got %v", patterns)
	}
	if patterns[0].Mode != "contains_char" || patterns[0].Value != "_" {
		t.Fatalf("unexpected first pattern: %+v", patterns[0])
	}
	if patterns[1].Mode != "contains_text" || patterns[1].Value != "TMP" {
		t.Fatalf("unexpected second pattern: %+v", patterns[1])
	}
}

func TestCollectTagOccurrences_ExcludesTagByContainsChar(t *testing.T) {
	obj1 := domain.GraphicObject{Id: "obj-1", TechnologyObject: []domain.TechnologyObject{{Id: "to-1", Tag: "A_B"}}}
	obj2 := domain.GraphicObject{Id: "obj-2", TechnologyObject: []domain.TechnologyObject{{Id: "to-2", Tag: "A_B"}}}
	obj3 := domain.GraphicObject{Id: "obj-3", TechnologyObject: []domain.TechnologyObject{{Id: "to-3", Tag: "SHARED"}}}
	obj4 := domain.GraphicObject{Id: "obj-4", TechnologyObject: []domain.TechnologyObject{{Id: "to-4", Tag: "SHARED"}}}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj1, obj2, obj3, obj4}, nil, nil)

	patterns := []duplicateTagPattern{{Mode: "contains_char", Value: "_"}}
	var all []TagOccurrence
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				all = append(all, collectTagOccurrencesForObject(sol, scheme.Id, scheme.Name, obj, nil, patterns)...)
			}
		}
	}
	for _, occ := range all {
		if strings.Contains(occ.Tag, "_") {
			t.Fatalf("tag with underscore should be excluded, got %+v", occ)
		}
	}
	violations, duplicateTags := BuildDuplicateTagIndex(all)
	if len(duplicateTags) != 1 || duplicateTags[0] != "SHARED" {
		t.Fatalf("expected only SHARED duplicate, got %v", duplicateTags)
	}
	if !violations["obj-3"] || !violations["obj-4"] {
		t.Fatalf("expected SHARED violations, got %v", violations)
	}
	if violations["obj-1"] || violations["obj-2"] {
		t.Fatalf("underscore tags should not create violations, got %v", violations)
	}
}

func TestCollectTagOccurrences_ExcludesTagByContainsText(t *testing.T) {
	obj := domain.GraphicObject{
		Id: "obj-1",
		TechnologyObject: []domain.TechnologyObject{{
			Id: "to-1", Tag: "MAIN",
			TechnologyProperty: []domain.TechnologyProperty{
				{Name: "P1", Tag: "TMP_VAL"},
				{Name: "P2", Tag: "KEEP"},
			},
		}},
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, nil, nil)
	patterns := []duplicateTagPattern{{Mode: "contains_text", Value: "TMP"}}
	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, nil, patterns)
	tags := map[string]bool{}
	for _, occ := range occurrences {
		tags[occ.Tag] = true
	}
	if tags["TMP_VAL"] {
		t.Fatal("contains_text TMP should exclude TMP_VAL")
	}
	if !tags["MAIN"] || !tags["KEEP"] {
		t.Fatalf("expected MAIN and KEEP, got %v", tags)
	}
}

func TestCollectTagOccurrences_ScopesAndTagPatternsIndependent(t *testing.T) {
	obj := domain.GraphicObject{
		Id: "obj-1",
		TechnologyObject: []domain.TechnologyObject{{
			Id: "to-1", Tag: "MAIN_X",
			TechnologyProperty: []domain.TechnologyProperty{{Name: "P1", Tag: "PROP"}},
		}},
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, []domain.ControlCommand{
		{Tag: "CMD", Target: "to-1", Template: "Open"},
	}, nil)

	excludedScopes := map[string]bool{"command": true}
	patterns := []duplicateTagPattern{{Mode: "contains_char", Value: "_"}}
	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, excludedScopes, patterns)
	tags := map[string]bool{}
	scopes := map[string]bool{}
	for _, occ := range occurrences {
		tags[occ.Tag] = true
		scopes[occ.Scope] = true
	}
	if tags["MAIN_X"] {
		t.Fatal("MAIN_X should be excluded by tag pattern")
	}
	if tags["CMD"] || scopes["command"] {
		t.Fatal("command scope should be excluded")
	}
	if !tags["PROP"] {
		t.Fatalf("property PROP should remain, got %v", tags)
	}
}

func TestGetBackgroundDuplicateScanResults_HydratesNodesWhenScannerRuleEmpty(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, _ := seedGraphicSolution(t, store, "obj-dup-a", "SHARED")

	projectID := uuid.New()
	schemeNodeID := uuid.New()
	if err := store.InsertNode(ctx, repository.NodeRow{
		ID: projectID, SolutionID: solutionID, NodeType: "project", XMLID: "proj-1", Name: "P1", SortIndex: 0,
	}); err != nil {
		t.Fatalf("InsertNode project: %v", err)
	}
	if err := store.InsertNode(ctx, repository.NodeRow{
		ID: schemeNodeID, SolutionID: solutionID, ParentID: &projectID, NodeType: "graphic_scheme",
		XMLID: "sch-1", Name: "Scheme", SortIndex: 0,
	}); err != nil {
		t.Fatalf("InsertNode scheme: %v", err)
	}

	ruleID := uuid.New()
	indication, err := json.Marshal(map[string]interface{}{"type": "solid", "color": "#F85149"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertComplianceRule(ctx, repository.ComplianceRule{
		ID: ruleID, UserID: repository.DefaultUserID, Template: "*", ParameterScope: "object",
		ValueSource: "field", TextCondition: "duplicate_tag", EmptyAllowed: true, Indication: indication,
	}); err != nil {
		t.Fatalf("InsertComplianceRule: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DeleteComplianceRule(context.Background(), repository.DefaultUserID, ruleID)
	})

	if err := store.UpsertComplianceTagCount(ctx, solutionID, "SHARED", 2); err != nil {
		t.Fatalf("UpsertComplianceTagCount: %v", err)
	}
	for _, objID := range []string{"obj-dup-a", "obj-dup-b"} {
		if err := store.UpsertComplianceDuplicateRow(ctx, repository.ComplianceDuplicateRow{
			SolutionID: solutionID, Tag: "SHARED", ObjectID: objID, ObjectName: objID, ObjectType: "Pump",
			SchemeXMLID: "sch-1", SchemeName: "Scheme", Scope: "object", ScopeLabel: "Основные", SourceLabel: "Tag",
		}); err != nil {
			t.Fatalf("UpsertComplianceDuplicateRow %s: %v", objID, err)
		}
	}

	result, err := svc.GetBackgroundDuplicateScanResults(ctx, solutionID)
	if err != nil {
		t.Fatalf("GetBackgroundDuplicateScanResults: %v", err)
	}
	if len(result.Rows) < 2 {
		t.Fatalf("expected persisted rows, got %d", len(result.Rows))
	}
	if !result.ViolationsByObject["obj-dup-a"] || !result.ViolationsByObject["obj-dup-b"] {
		t.Fatalf("expected violations for both objects, got %v", result.ViolationsByObject)
	}
	if len(result.Nodes) == 0 {
		t.Fatal("expected explorer nodes when scanner.rule_id is empty but duplicate_tag rule exists")
	}
	foundScheme := false
	for _, n := range result.Nodes {
		if n.XMLID == "sch-1" && n.Type == "graphic_scheme" && len(n.Indicators) > 0 {
			foundScheme = true
			if n.Indicators[0].RuleID != ruleID.String() {
				t.Fatalf("expected rule_id %s on scheme node, got %s", ruleID, n.Indicators[0].RuleID)
			}
		}
	}
	if !foundScheme {
		t.Fatalf("expected graphic_scheme node for sch-1, got %+v", result.Nodes)
	}
}

func TestResolveDuplicateResultsRuleID_FallsBackToCompletedJob(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, _ := seedGraphicSolution(t, store, "obj-job", "JOB-TAG")

	rules, err := store.ListComplianceRules(ctx, repository.DefaultUserID, nil)
	if err != nil {
		t.Fatalf("ListComplianceRules: %v", err)
	}
	for _, r := range rules {
		if isDuplicateTagRule(r) {
			_ = store.DeleteComplianceRule(ctx, repository.DefaultUserID, r.ID)
		}
	}

	jobRuleID := uuid.New()
	jobID := uuid.New()
	finished := time.Now()
	if err := store.InsertComplianceDuplicateJob(ctx, repository.ComplianceDuplicateJob{
		ID: jobID, SolutionID: solutionID, RuleID: jobRuleID, Status: "completed",
		TotalObjects: 1, ProcessedObjects: 1, FinishedAt: &finished,
	}); err != nil {
		t.Fatalf("InsertComplianceDuplicateJob: %v", err)
	}

	resolved, err := svc.resolveDuplicateResultsRuleID(ctx, solutionID, uuid.Nil)
	if err != nil {
		t.Fatalf("resolveDuplicateResultsRuleID: %v", err)
	}
	if resolved != jobRuleID {
		t.Fatalf("expected job rule_id %s, got %s", jobRuleID, resolved)
	}
}

func TestBuildDuplicateScanResultFromStore_NilRuleUsesDuplicateTagRule(t *testing.T) {
	svc, store := openChangedObjectsTestEnv(t)
	ctx := context.Background()
	solutionID, _ := seedGraphicSolution(t, store, "obj-nil-a", "DUP")

	projectID := uuid.New()
	if err := store.InsertNode(ctx, repository.NodeRow{
		ID: projectID, SolutionID: solutionID, NodeType: "project", XMLID: "proj-1", Name: "P1",
	}); err != nil {
		t.Fatalf("InsertNode project: %v", err)
	}
	if err := store.InsertNode(ctx, repository.NodeRow{
		ID: uuid.New(), SolutionID: solutionID, ParentID: &projectID, NodeType: "graphic_scheme",
		XMLID: "sch-1", Name: "Scheme",
	}); err != nil {
		t.Fatalf("InsertNode scheme: %v", err)
	}

	ruleID := uuid.New()
	indication, _ := json.Marshal(map[string]interface{}{"type": "pulse", "color": "#AABBCC"})
	if err := store.InsertComplianceRule(ctx, repository.ComplianceRule{
		ID: ruleID, UserID: repository.DefaultUserID, Template: "*", ParameterScope: "object",
		ValueSource: "field", TextCondition: "duplicate_tag", EmptyAllowed: true, Indication: indication,
	}); err != nil {
		t.Fatalf("InsertComplianceRule: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DeleteComplianceRule(context.Background(), repository.DefaultUserID, ruleID)
	})

	_ = store.UpsertComplianceTagCount(ctx, solutionID, "DUP", 2)
	_ = store.UpsertComplianceDuplicateRow(ctx, repository.ComplianceDuplicateRow{
		SolutionID: solutionID, Tag: "DUP", ObjectID: "obj-nil-a", SchemeXMLID: "sch-1",
		Scope: "object", ScopeLabel: "Основные", SourceLabel: "Tag",
	})
	_ = store.UpsertComplianceDuplicateRow(ctx, repository.ComplianceDuplicateRow{
		SolutionID: solutionID, Tag: "DUP", ObjectID: "obj-nil-b", SchemeXMLID: "sch-1",
		Scope: "object", ScopeLabel: "Основные", SourceLabel: "Tag",
	})

	result, err := svc.buildDuplicateScanResultFromStore(ctx, solutionID, uuid.Nil)
	if err != nil {
		t.Fatalf("buildDuplicateScanResultFromStore: %v", err)
	}
	if len(result.Nodes) == 0 {
		t.Fatal("expected nodes when ruleID is nil and duplicate_tag rule exists")
	}
}

func TestCollectTagOccurrences_TechnologyObjectIDDistinctFromGraphic(t *testing.T) {
	obj := domain.GraphicObject{
		Id: "graphic-id-1",
		TechnologyObject: []domain.TechnologyObject{{
			Id: "tech-id-99", Tag: "PUMP-TAG",
		}},
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, nil, nil)
	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, nil, nil)
	if len(occurrences) == 0 {
		t.Fatal("expected at least one occurrence")
	}
	for _, occ := range occurrences {
		if occ.ObjectID != "graphic-id-1" {
			t.Fatalf("object_id must stay graphic Id, got %q", occ.ObjectID)
		}
		if occ.TechnologyObjectID != "tech-id-99" {
			t.Fatalf("technology_object_id must be TO Id, got %q", occ.TechnologyObjectID)
		}
	}
}

func TestCollectTagOccurrences_EmptyTechnologyObjectID(t *testing.T) {
	obj := domain.GraphicObject{
		Id: "graphic-only",
		Tag: "G-TAG",
	}
	sol := makeDuplicateScanSolution([]domain.GraphicObject{obj}, nil, nil)
	occurrences := collectTagOccurrencesForObject(sol, "scheme-1", "Scheme 1", obj, nil, nil)
	if len(occurrences) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occurrences))
	}
	if occurrences[0].ObjectID != "graphic-only" {
		t.Fatalf("object_id=%q", occurrences[0].ObjectID)
	}
	if occurrences[0].TechnologyObjectID != "" {
		t.Fatalf("expected empty technology_object_id, got %q", occurrences[0].TechnologyObjectID)
	}
}

func TestResolveObjectField_IdUsesTechnologyOnly(t *testing.T) {
	obj := domain.GraphicObject{Id: "graphic-id-1"}
	if got := resolveObjectField("Id", &obj, nil); got != "" {
		t.Fatalf("nil TO: expected empty Id, got %q", got)
	}
	emptyTO := &domain.TechnologyObject{Id: ""}
	if got := resolveObjectField("Id", &obj, emptyTO); got != "" {
		t.Fatalf("empty TO Id: expected empty, got %q (must not fall back to graphic)", got)
	}
	to := &domain.TechnologyObject{Id: "tech-id-99"}
	if got := resolveObjectField("Id", &obj, to); got != "tech-id-99" {
		t.Fatalf("expected tech-id-99, got %q", got)
	}
}
