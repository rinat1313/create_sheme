package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDuplicateScanActive = errors.New("duplicate scan already running for this solution")
	ErrDuplicateNoRule     = errors.New("no duplicate tag rule configured")
	ErrDuplicateRuleExists = errors.New("only one duplicate tag rule allowed")
)

var duplicateScanJobLocks sync.Map

type TagOccurrence struct {
	Tag                string `json:"tag"`
	ObjectID           string `json:"object_id"`
	TechnologyObjectID string `json:"technology_object_id"`
	ObjectName         string `json:"object_name"`
	ObjectType         string `json:"object_type"`
	SchemeXMLID        string `json:"scheme_xml_id"`
	SchemeName         string `json:"scheme_name"`
	Scope              string `json:"scope"`
	ScopeLabel         string `json:"scope_label"`
	SourceLabel        string `json:"source_label"`
	PropertyID         string `json:"property_id,omitempty"`
	EntityID           string `json:"entity_id,omitempty"`
}

type DuplicateScanResult struct {
	ViolationsByObject map[string]bool      `json:"violations_by_object"`
	DuplicateTags      []string             `json:"duplicate_tags"`
	Nodes              []ComplianceNodeHint `json:"nodes"`
	Rows               []TagOccurrence      `json:"rows"`
}

type PatchDuplicateTagRequest struct {
	Scope       string `json:"scope"`
	ObjectID    string `json:"object_id"`
	SchemeXMLID string `json:"scheme_xml_id"`
	PropertyID  string `json:"property_id"`
	EntityID    string `json:"entity_id"`
	NewTag      string `json:"new_tag"`
}

func scopeLabel(scope string) string {
	switch scope {
	case "property":
		return "Характеристики"
	case "command":
		return "Команда"
	case "protection":
		return "Защита"
	default:
		return "Основные"
	}
}

func objectDisplayName(obj domain.GraphicObject, to *domain.TechnologyObject) string {
	if to != nil && to.Name != "" {
		return to.Name
	}
	return obj.Name
}

func objectDisplayTag(obj domain.GraphicObject, to *domain.TechnologyObject) string {
	if to != nil && to.Tag != "" {
		return to.Tag
	}
	return obj.Tag
}

func objectDisplayType(obj domain.GraphicObject, to *domain.TechnologyObject) string {
	if to != nil && to.Template != "" {
		return to.Template
	}
	return obj.TechnologyTemplate()
}

func technologyObjectDisplayID(to *domain.TechnologyObject) string {
	if to == nil {
		return ""
	}
	return strings.TrimSpace(to.Id)
}

func tagOccurrenceFromDuplicateRow(r repository.ComplianceDuplicateRow) TagOccurrence {
	return TagOccurrence{
		Tag:                r.Tag,
		ObjectID:           r.ObjectID,
		TechnologyObjectID: strings.TrimSpace(r.TechnologyObjectID),
		ObjectName:         r.ObjectName,
		ObjectType:         r.ObjectType,
		SchemeXMLID:        r.SchemeXMLID,
		SchemeName:         r.SchemeName,
		Scope:              r.Scope,
		ScopeLabel:         r.ScopeLabel,
		SourceLabel:        r.SourceLabel,
		PropertyID:         r.PropertyID,
		EntityID:           r.EntityID,
	}
}

func complianceDuplicateRowFromOccurrence(solutionID uuid.UUID, occ TagOccurrence) repository.ComplianceDuplicateRow {
	return repository.ComplianceDuplicateRow{
		SolutionID:         solutionID,
		Tag:                occ.Tag,
		ObjectID:           occ.ObjectID,
		TechnologyObjectID: occ.TechnologyObjectID,
		ObjectName:         occ.ObjectName,
		ObjectType:         occ.ObjectType,
		SchemeXMLID:        occ.SchemeXMLID,
		SchemeName:         occ.SchemeName,
		Scope:              occ.Scope,
		ScopeLabel:         occ.ScopeLabel,
		SourceLabel:        occ.SourceLabel,
		PropertyID:         occ.PropertyID,
		EntityID:           occ.EntityID,
	}
}

func propertySourceLabel(p domain.TechnologyProperty) string {
	if p.Name != "" {
		return p.Name
	}
	if p.Template != "" {
		return p.Template
	}
	return "Tag"
}

type duplicateTagPattern struct {
	Mode  string
	Value string
}

func parseDuplicateExcludedScopes(rule repository.ComplianceRule) map[string]bool {
	start := time.Now()
	defer func() {
		slog.Debug("parseDuplicateExcludedScopes done", "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("parseDuplicateExcludedScopes start")

	excluded := map[string]bool{}
	ind := parseIndicationJSON(rule.Indication)
	raw, ok := ind["excluded_scopes"]
	if !ok {
		return excluded
	}
	items, ok := raw.([]interface{})
	if !ok {
		return excluded
	}
	for _, item := range items {
		scope, ok := item.(string)
		if !ok || scope == "" {
			continue
		}
		excluded[scope] = true
	}
	return excluded
}

func parseDuplicateExcludedTagPatterns(rule repository.ComplianceRule) []duplicateTagPattern {
	start := time.Now()
	var patterns []duplicateTagPattern
	defer func() {
		slog.Debug("parseDuplicateExcludedTagPatterns done",
			"count", len(patterns),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("parseDuplicateExcludedTagPatterns start")

	ind := parseIndicationJSON(rule.Indication)
	raw, ok := ind["excluded_tag_patterns"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		mode, _ := m["mode"].(string)
		value, _ := m["value"].(string)
		mode = strings.TrimSpace(mode)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch mode {
		case "contains_char", "exact_one_char", "first_char":
			if utf8.RuneCountInString(value) != 1 {
				continue
			}
		case "contains_text":
		default:
			continue
		}
		patterns = append(patterns, duplicateTagPattern{Mode: mode, Value: value})
	}
	return patterns
}

func duplicateScopeIncluded(excluded map[string]bool, scope string) bool {
	if len(excluded) == 0 {
		return true
	}
	return !excluded[scope]
}

func tagExcludedByPatterns(tag string, patterns []duplicateTagPattern) bool {
	if tag == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p.Value == "" {
			continue
		}
		switch p.Mode {
		case "contains_char", "contains_text":
			if strings.Contains(tag, p.Value) {
				return true
			}
		case "exact_one_char":
			if utf8.RuneCountInString(tag) == 1 && tag == p.Value {
				return true
			}
		case "first_char":
			tr, _ := utf8.DecodeRuneInString(tag)
			vr, _ := utf8.DecodeRuneInString(p.Value)
			if tr != utf8.RuneError && tr == vr {
				return true
			}
		}
	}
	return false
}

func (s *SolutionService) findDuplicateTagRule(ctx context.Context, ruleID uuid.UUID) (repository.ComplianceRule, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("findDuplicateTagRule done",
			"rule_id", ruleID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("findDuplicateTagRule start", "rule_id", ruleID)

	rules, err := s.store.ListComplianceRules(ctx, UserIDFromContext(ctx), nil)
	if err != nil {
		return repository.ComplianceRule{}, err
	}
	if ruleID != uuid.Nil {
		for _, r := range rules {
			if r.ID == ruleID && isDuplicateTagRule(r) {
				return r, nil
			}
		}
		err = ErrDuplicateNoRule
		return repository.ComplianceRule{}, err
	}
	for _, r := range rules {
		if isDuplicateTagRule(r) {
			return r, nil
		}
	}
	err = ErrDuplicateNoRule
	return repository.ComplianceRule{}, err
}

func (s *SolutionService) resolveDuplicateResultsRuleID(ctx context.Context, solutionID, scannerRuleID uuid.UUID) (uuid.UUID, error) {
	start := time.Now()
	var err error
	var resolved uuid.UUID
	defer func() {
		slog.Info("resolveDuplicateResultsRuleID done",
			"solution_id", solutionID,
			"scanner_rule_id", scannerRuleID,
			"resolved_rule_id", resolved,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("resolveDuplicateResultsRuleID start", "solution_id", solutionID, "scanner_rule_id", scannerRuleID)

	if scannerRuleID != uuid.Nil {
		resolved = scannerRuleID
		return resolved, nil
	}
	rule, ferr := s.findDuplicateTagRule(ctx, uuid.Nil)
	if ferr == nil {
		resolved = rule.ID
		return resolved, nil
	}
	jobRuleID, jerr := s.store.GetLatestCompletedComplianceDuplicateJobRuleID(ctx, solutionID)
	if jerr != nil {
		if errors.Is(jerr, pgx.ErrNoRows) {
			err = ferr
			return uuid.Nil, err
		}
		err = jerr
		return uuid.Nil, err
	}
	resolved = jobRuleID
	return resolved, nil
}

func (s *SolutionService) countDuplicateTagRules(ctx context.Context, excludeID *uuid.UUID) (int, error) {
	rules, err := s.store.ListComplianceRules(ctx, UserIDFromContext(ctx), nil)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range rules {
		if !isDuplicateTagRule(r) {
			continue
		}
		if excludeID != nil && r.ID == *excludeID {
			continue
		}
		count++
	}
	return count, nil
}

func collectTagOccurrencesForObject(sol domain.Solution, schemeXMLID, schemeName string, obj domain.GraphicObject, excludedScopes map[string]bool, excludedTagPatterns []duplicateTagPattern) []TagOccurrence {
	start := time.Now()
	defer func() {
		slog.Debug("collectTagOccurrencesForObject done", "object_id", obj.Id, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("collectTagOccurrencesForObject start", "object_id", obj.Id)

	var occurrences []TagOccurrence
	var to *domain.TechnologyObject
	if len(obj.TechnologyObject) > 0 {
		to = &obj.TechnologyObject[0]
	}
	objName := objectDisplayName(obj, to)
	objType := objectDisplayType(obj, to)
	toID := technologyObjectDisplayID(to)

	mainTag := strings.TrimSpace(resolveObjectField("Tag", &obj, to))
	if mainTag != "" && duplicateScopeIncluded(excludedScopes, "object") && !tagExcludedByPatterns(mainTag, excludedTagPatterns) {
		occurrences = append(occurrences, TagOccurrence{
			Tag: mainTag, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
			SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "object",
			ScopeLabel: scopeLabel("object"), SourceLabel: "Tag",
		})
	}

	if to != nil && duplicateScopeIncluded(excludedScopes, "property") {
		for _, p := range to.TechnologyProperty {
			pt := strings.TrimSpace(p.Tag)
			if pt != "" && !tagExcludedByPatterns(pt, excludedTagPatterns) {
				occurrences = append(occurrences, TagOccurrence{
					Tag: pt, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
					SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "property",
					ScopeLabel: scopeLabel("property"), SourceLabel: propertySourceLabel(p),
					PropertyID: p.Id,
				})
			}
		}
	}

	if duplicateScopeIncluded(excludedScopes, "command") {
		for _, cmd := range sol.ControlCommands.ControlCommand {
			if !matchesTarget(cmd.Target, obj) {
				continue
			}
			cmdLabel := cmd.Template
			if cmdLabel == "" {
				cmdLabel = cmd.Name
			}
			ct := strings.TrimSpace(cmd.Tag)
			if ct != "" && !tagExcludedByPatterns(ct, excludedTagPatterns) {
				occurrences = append(occurrences, TagOccurrence{
					Tag: ct, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
					SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "command",
					ScopeLabel: scopeLabel("command"), SourceLabel: cmdLabel, EntityID: cmd.Id,
				})
			}
			for _, p := range cmd.TechnologyProperty {
				pt := strings.TrimSpace(p.Tag)
				if pt != "" && !tagExcludedByPatterns(pt, excludedTagPatterns) {
					occurrences = append(occurrences, TagOccurrence{
						Tag: pt, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
						SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "command",
						ScopeLabel: scopeLabel("command"), SourceLabel: propertySourceLabel(p),
						PropertyID: p.Id, EntityID: cmd.Id,
					})
				}
			}
		}
	}

	if duplicateScopeIncluded(excludedScopes, "protection") {
		for _, auto := range sol.Automations.Automation {
			if !matchesTarget(auto.Target, obj) {
				continue
			}
			autoLabel := auto.Template
			if autoLabel == "" {
				autoLabel = auto.Name
			}
			at := strings.TrimSpace(auto.Tag)
			if at != "" && !tagExcludedByPatterns(at, excludedTagPatterns) {
				occurrences = append(occurrences, TagOccurrence{
					Tag: at, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
					SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "protection",
					ScopeLabel: scopeLabel("protection"), SourceLabel: autoLabel, EntityID: auto.Id,
				})
			}
			for _, p := range auto.TechnologyProperty {
				pt := strings.TrimSpace(p.Tag)
				if pt != "" && !tagExcludedByPatterns(pt, excludedTagPatterns) {
					occurrences = append(occurrences, TagOccurrence{
						Tag: pt, ObjectID: obj.Id, TechnologyObjectID: toID, ObjectName: objName, ObjectType: objType,
						SchemeXMLID: schemeXMLID, SchemeName: schemeName, Scope: "protection",
						ScopeLabel: scopeLabel("protection"), SourceLabel: propertySourceLabel(p),
						PropertyID: p.Id, EntityID: auto.Id,
					})
				}
			}
		}
	}

	return occurrences
}

func collectAllTagOccurrencesParallel(sol domain.Solution, refs []tagEditObjectRef, schemeNames map[string]string, excludedScopes map[string]bool, excludedTagPatterns []duplicateTagPattern, onProcessed func()) []TagOccurrence {
	start := time.Now()
	defer func() {
		slog.Info("collectAllTagOccurrencesParallel done", "refs", len(refs), "duration_ms", time.Since(start).Milliseconds())
	}()
	if len(refs) == 0 {
		return nil
	}

	workers := parallelWorkersForJobs(len(refs))
	slog.Debug("collectAllTagOccurrencesParallel start", "refs", len(refs), "workers", workers, "num_cpu", runtime.NumCPU())

	jobs := make(chan int, len(refs))
	for i := range refs {
		jobs <- i
	}
	close(jobs)

	results := make([][]TagOccurrence, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []TagOccurrence
			for idx := range jobs {
				ref := refs[idx]
				schemeName := schemeNames[ref.schemeXMLID]
				local = append(local, collectTagOccurrencesForObject(sol, ref.schemeXMLID, schemeName, ref.obj, excludedScopes, excludedTagPatterns)...)
				if onProcessed != nil {
					onProcessed()
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	var allOccurrences []TagOccurrence
	for _, local := range results {
		allOccurrences = append(allOccurrences, local...)
	}
	return allOccurrences
}

func BuildDuplicateTagIndex(occurrences []TagOccurrence) (map[string]bool, []string) {
	start := time.Now()
	defer func() {
		slog.Debug("BuildDuplicateTagIndex done", "occurrences", len(occurrences), "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("BuildDuplicateTagIndex start", "occurrences", len(occurrences))

	tagIndex := map[string][]TagOccurrence{}
	for _, occ := range occurrences {
		tagIndex[occ.Tag] = append(tagIndex[occ.Tag], occ)
	}

	violations := map[string]bool{}
	var duplicateTags []string
	for tag, occs := range tagIndex {
		if len(occs) < 2 {
			continue
		}
		duplicateTags = append(duplicateTags, tag)
		for _, occ := range occs {
			violations[occ.ObjectID] = true
		}
	}
	sort.Strings(duplicateTags)
	return violations, duplicateTags
}

func BuildDuplicateTableRows(allOccurrences []TagOccurrence, duplicateTags []string) []TagOccurrence {
	start := time.Now()
	defer func() {
		slog.Debug("BuildDuplicateTableRows done", "rows", len(allOccurrences), "duration_ms", time.Since(start).Milliseconds())
	}()
	dupSet := map[string]bool{}
	for _, tag := range duplicateTags {
		dupSet[tag] = true
	}
	var rows []TagOccurrence
	for _, occ := range allOccurrences {
		if dupSet[occ.Tag] {
			rows = append(rows, occ)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Tag != rows[j].Tag {
			return rows[i].Tag < rows[j].Tag
		}
		if rows[i].ObjectID != rows[j].ObjectID {
			return rows[i].ObjectID < rows[j].ObjectID
		}
		return rows[i].SourceLabel < rows[j].SourceLabel
	})
	return rows
}

func (s *SolutionService) BuildDuplicateScanResult(ctx context.Context, sol domain.Solution, solutionID uuid.UUID, rule repository.ComplianceRule, violations map[string]bool, duplicateTags []string, allOccurrences []TagOccurrence) (DuplicateScanResult, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("BuildDuplicateScanResult done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	_ = sol

	rows := BuildDuplicateTableRows(allOccurrences, duplicateTags)
	if len(violations) == 0 {
		return DuplicateScanResult{
			ViolationsByObject: violations,
			DuplicateTags:      duplicateTags,
			Nodes:              []ComplianceNodeHint{},
			Rows:               rows,
		}, nil
	}

	schemeXMLIDs := map[string]bool{}
	for _, occ := range rows {
		if occ.SchemeXMLID != "" {
			schemeXMLIDs[occ.SchemeXMLID] = true
		}
	}
	for _, occ := range allOccurrences {
		if violations[occ.ObjectID] && occ.SchemeXMLID != "" {
			schemeXMLIDs[occ.SchemeXMLID] = true
		}
	}

	hints, err := s.buildDuplicateNodesFromSchemeIDs(ctx, solutionID, rule, schemeXMLIDs)
	if err != nil {
		return DuplicateScanResult{}, err
	}

	return DuplicateScanResult{
		ViolationsByObject: violations,
		DuplicateTags:      duplicateTags,
		Nodes:              hints,
		Rows:               rows,
	}, nil
}

func (s *SolutionService) buildDuplicateNodesFromSchemeIDs(ctx context.Context, solutionID uuid.UUID, rule repository.ComplianceRule, schemeXMLIDs map[string]bool) ([]ComplianceNodeHint, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("buildDuplicateNodesFromSchemeIDs done",
			"solution_id", solutionID,
			"schemes", len(schemeXMLIDs),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("buildDuplicateNodesFromSchemeIDs start", "solution_id", solutionID, "schemes", len(schemeXMLIDs))

	if len(schemeXMLIDs) == 0 {
		return []ComplianceNodeHint{}, nil
	}

	nodes, err := s.store.ListNodes(ctx, solutionID)
	if err != nil {
		return nil, err
	}

	nodeByXML := map[string]repository.NodeRow{}
	for _, n := range nodes {
		nodeByXML[n.XMLID] = n
	}

	ind := parseIndicationJSON(rule.Indication)
	indicator := ComplianceIndicator{
		RuleID: rule.ID.String(),
		Color:  indicationColor(ind),
		Type:   indicationType(ind),
	}

	schemeIndicators := map[string][]ComplianceIndicator{}
	projectRuleSets := map[string]map[string]ComplianceIndicator{}
	folderRuleSets := map[string]map[string]ComplianceIndicator{}

	for xmlID := range schemeXMLIDs {
		schemeIndicators[xmlID] = []ComplianceIndicator{indicator}
		n, ok := nodeByXML[xmlID]
		if !ok || n.ParentID == nil {
			continue
		}
		parent, ok := findNodeByID(nodes, *n.ParentID)
		if !ok {
			continue
		}
		if parent.NodeType == "project" {
			if projectRuleSets[parent.XMLID] == nil {
				projectRuleSets[parent.XMLID] = map[string]ComplianceIndicator{}
			}
			projectRuleSets[parent.XMLID][indicator.RuleID] = indicator
			if parent.ParentID != nil {
				folder, ok := findNodeByID(nodes, *parent.ParentID)
				if ok && folder.NodeType == "folder" {
					if folderRuleSets[folder.XMLID] == nil {
						folderRuleSets[folder.XMLID] = map[string]ComplianceIndicator{}
					}
					folderRuleSets[folder.XMLID][indicator.RuleID] = indicator
				}
			}
		} else if parent.NodeType == "folder" {
			if folderRuleSets[parent.XMLID] == nil {
				folderRuleSets[parent.XMLID] = map[string]ComplianceIndicator{}
			}
			folderRuleSets[parent.XMLID][indicator.RuleID] = indicator
		}
	}

	var hints []ComplianceNodeHint
	for xmlID, indicators := range schemeIndicators {
		n := nodeByXML[xmlID]
		hints = append(hints, ComplianceNodeHint{
			XMLID: xmlID, Type: "graphic_scheme", Name: n.Name, Indicators: indicators,
		})
	}
	for xmlID, ruleSet := range projectRuleSets {
		n := nodeByXML[xmlID]
		hints = append(hints, ComplianceNodeHint{
			XMLID: xmlID, Type: "project", Name: n.Name, Indicators: mapValues(ruleSet),
		})
	}
	for xmlID, ruleSet := range folderRuleSets {
		n := nodeByXML[xmlID]
		hints = append(hints, ComplianceNodeHint{
			XMLID: xmlID, Type: "folder", Name: n.Name, Indicators: mapValues(ruleSet),
		})
	}
	return hints, nil
}

func (s *SolutionService) buildDuplicateScanResultFromStore(ctx context.Context, solutionID, ruleID uuid.UUID) (DuplicateScanResult, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("buildDuplicateScanResultFromStore done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("buildDuplicateScanResultFromStore start", "solution_id", solutionID)

	dbRows, err := s.store.ListComplianceDuplicateRows(ctx, solutionID)
	if err != nil {
		return DuplicateScanResult{}, err
	}
	duplicateTags, err := s.store.ListDuplicateTags(ctx, solutionID)
	if err != nil {
		return DuplicateScanResult{}, err
	}

	rows := make([]TagOccurrence, 0, len(dbRows))
	violations := map[string]bool{}
	schemeXMLIDs := map[string]bool{}
	for _, r := range dbRows {
		rows = append(rows, tagOccurrenceFromDuplicateRow(r))
		violations[r.ObjectID] = true
		if r.SchemeXMLID != "" {
			schemeXMLIDs[r.SchemeXMLID] = true
		}
	}

	result := DuplicateScanResult{
		ViolationsByObject: violations,
		DuplicateTags:      duplicateTags,
		Nodes:              []ComplianceNodeHint{},
		Rows:               rows,
	}
	if len(violations) == 0 {
		return result, nil
	}

	rule, rerr := s.findDuplicateTagRule(ctx, ruleID)
	if rerr != nil {
		return result, nil
	}
	hints, herr := s.buildDuplicateNodesFromSchemeIDs(ctx, solutionID, rule, schemeXMLIDs)
	if herr != nil {
		return result, nil
	}
	result.Nodes = hints
	return result, nil
}

func (s *SolutionService) collectAllGraphicObjectRefs(sol domain.Solution) []tagEditObjectRef {
	return s.collectGraphicObjects(sol)
}

func (s *SolutionService) isTagEditRunning(ctx context.Context, solutionID uuid.UUID) bool {
	if _, loaded := tagEditJobLocks.Load(solutionID.String()); loaded {
		return true
	}
	active, err := s.store.HasActiveTagEditJob(ctx, solutionID)
	return err == nil && active
}

func (s *SolutionService) StartDuplicateScanJob(ctx context.Context, solutionID, ruleID uuid.UUID, sessionID *uuid.UUID) (uuid.UUID, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("StartDuplicateScanJob done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Info("StartDuplicateScanJob start", "solution_id", solutionID)

	rule, err := s.findDuplicateTagRule(ctx, ruleID)
	if err != nil {
		return uuid.Nil, err
	}

	if s.isTagEditRunning(ctx, solutionID) {
		err = ErrTagEditJobActive
		return uuid.Nil, err
	}

	if s.isBackgroundScanEnabled(ctx, solutionID) {
		err = ErrBackgroundScanActive
		return uuid.Nil, err
	}

	key := solutionID.String()
	if _, loaded := duplicateScanJobLocks.LoadOrStore(key, true); loaded {
		err = ErrDuplicateScanActive
		return uuid.Nil, err
	}

	total, countErr := s.store.CountSolutionObjects(ctx, solutionID)
	if countErr != nil {
		total = 0
	}

	jobID := uuid.New()
	job := repository.ComplianceDuplicateJob{
		ID:           jobID,
		SolutionID:   solutionID,
		RuleID:       ruleID,
		Status:       "pending",
		TotalObjects: total,
	}
	if err = s.store.InsertComplianceDuplicateJob(ctx, job); err != nil {
		duplicateScanJobLocks.Delete(key)
		return uuid.Nil, err
	}

	go s.runDuplicateScanJob(jobID, solutionID, rule, sessionID)

	return jobID, nil
}

func (s *SolutionService) buildSchemeNames(ctx context.Context, solutionID uuid.UUID, refs []tagEditObjectRef) map[string]string {
	start := time.Now()
	defer func() {
		slog.Debug("buildSchemeNames done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("buildSchemeNames start", "solution_id", solutionID)

	schemeNames := map[string]string{}
	for _, ref := range refs {
		if ref.schemeXMLID != "" && schemeNames[ref.schemeXMLID] == "" {
			schemeNames[ref.schemeXMLID] = ref.schemeXMLID
		}
	}
	nodes, err := s.store.ListNodes(ctx, solutionID)
	if err == nil {
		for _, n := range nodes {
			if n.NodeType == "graphic_scheme" {
				schemeNames[n.XMLID] = n.Name
			}
		}
	}
	return schemeNames
}

func collectAllTagOccurrencesSequential(sol domain.Solution, refs []tagEditObjectRef, schemeNames map[string]string, excludedScopes map[string]bool, excludedTagPatterns []duplicateTagPattern, onProcessed func()) []TagOccurrence {
	start := time.Now()
	defer func() {
		slog.Info("collectAllTagOccurrencesSequential done", "refs", len(refs), "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("collectAllTagOccurrencesSequential start", "refs", len(refs))

	var all []TagOccurrence
	for _, ref := range refs {
		schemeName := schemeNames[ref.schemeXMLID]
		all = append(all, collectTagOccurrencesForObject(sol, ref.schemeXMLID, schemeName, ref.obj, excludedScopes, excludedTagPatterns)...)
		if onProcessed != nil {
			onProcessed()
		}
	}
	return all
}

func (s *SolutionService) runDuplicateScanCore(ctx context.Context, sol domain.Solution, solutionID uuid.UUID, rule repository.ComplianceRule, onProcessed func()) (DuplicateScanResult, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("runDuplicateScanCore done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("runDuplicateScanCore start", "solution_id", solutionID)

	refs := s.collectAllGraphicObjectRefs(sol)
	schemeNames := s.buildSchemeNames(ctx, solutionID, refs)
	excludedScopes := parseDuplicateExcludedScopes(rule)
	excludedTagPatterns := parseDuplicateExcludedTagPatterns(rule)
	allOccurrences := collectAllTagOccurrencesSequential(sol, refs, schemeNames, excludedScopes, excludedTagPatterns, onProcessed)
	violations, duplicateTags := BuildDuplicateTagIndex(allOccurrences)
	result, err := s.BuildDuplicateScanResult(ctx, sol, solutionID, rule, violations, duplicateTags, allOccurrences)
	return result, err
}

func (s *SolutionService) RunDuplicateScanSync(ctx context.Context, solutionID, ruleID uuid.UUID, sessionID *uuid.UUID) (DuplicateScanResult, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("RunDuplicateScanSync done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Info("RunDuplicateScanSync start", "solution_id", solutionID)

	rule, err := s.findDuplicateTagRule(ctx, ruleID)
	if err != nil {
		return DuplicateScanResult{}, err
	}

	if s.isTagEditRunning(ctx, solutionID) {
		err = ErrTagEditJobActive
		return DuplicateScanResult{}, err
	}

	if s.isBackgroundScanEnabled(ctx, solutionID) {
		err = ErrBackgroundScanActive
		return DuplicateScanResult{}, err
	}

	sol, err := s.loadSolutionWithChanges(ctx, solutionID, sessionID)
	if err != nil {
		return DuplicateScanResult{}, err
	}
	return s.runDuplicateScanCore(ctx, sol, solutionID, rule, nil)
}

func (s *SolutionService) clearNormalizedDuplicateStore(ctx context.Context, solutionID uuid.UUID) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("clearNormalizedDuplicateStore done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	if err = s.store.ClearComplianceTagCounts(ctx, solutionID); err != nil {
		return err
	}
	if err = s.store.ClearComplianceTagObjectTags(ctx, solutionID); err != nil {
		return err
	}
	if err = s.store.ClearComplianceDuplicateRows(ctx, solutionID); err != nil {
		return err
	}
	if err = s.store.ClearComplianceDuplicateDirty(ctx, solutionID); err != nil {
		return err
	}
	return nil
}

func (s *SolutionService) runForcedDuplicateScanMemoryFirst(
	ctx context.Context,
	jobID, solutionID uuid.UUID,
	sol domain.Solution,
	rule repository.ComplianceRule,
	onProcessed func(),
) (int, int, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("runForcedDuplicateScanMemoryFirst done",
			"job_id", jobID,
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Info("runForcedDuplicateScanMemoryFirst start", "job_id", jobID, "solution_id", solutionID)

	if err = s.clearNormalizedDuplicateStore(ctx, solutionID); err != nil {
		return 0, 0, err
	}

	refs := s.collectAllGraphicObjectRefs(sol)
	schemeNames := s.buildSchemeNames(ctx, solutionID, refs)
	excluded := parseDuplicateExcludedScopes(rule)
	excludedTagPatterns := parseDuplicateExcludedTagPatterns(rule)
	engine := newTagCountEngine()
	occByObject := map[string][]TagOccurrence{}

	workers, batchSize, sleep := forcedScanThrottleConfig()
	_ = workers

	for i := 0; i < len(refs); i += batchSize {
		end := i + batchSize
		if end > len(refs) {
			end = len(refs)
		}
		batch := refs[i:end]
		for _, ref := range batch {
			schemeName := schemeNames[ref.schemeXMLID]
			newOccs := collectTagOccurrencesForObject(sol, ref.schemeXMLID, schemeName, ref.obj, excluded, excludedTagPatterns)
			oldOccs := occByObject[ref.obj.Id]
			_, became, lost := engine.adjustObject(ref.obj.Id, newOccs)
			occByObject[ref.obj.Id] = newOccs
			if perr := s.persistObjectTagAdjust(ctx, solutionID, ref.obj.Id, engine, oldOccs, newOccs, became, lost, occByObject); perr != nil {
				slog.Warn("persistObjectTagAdjust failed", "solution_id", solutionID, "object_id", ref.obj.Id, "err", perr)
			}
			if onProcessed != nil {
				onProcessed()
			}
		}
		if end < len(refs) && sleep > 0 {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				return len(engine.duplicateTags()), len(engine.duplicateRows()), err
			case <-time.After(sleep):
			}
		}
	}

	dupTags := engine.duplicateTags()
	dupSet := map[string]bool{}
	for _, tag := range dupTags {
		dupSet[tag] = true
	}
	violated := map[string]bool{}
	for _, occs := range occByObject {
		for _, occ := range occs {
			if dupSet[occ.Tag] {
				violated[occ.ObjectID] = true
			}
		}
	}
	return len(dupTags), len(violated), nil
}

func (s *SolutionService) runDuplicateScanJob(jobID, solutionID uuid.UUID, rule repository.ComplianceRule, sessionID *uuid.UUID) {
	start := time.Now()
	defer func() {
		duplicateScanJobLocks.Delete(solutionID.String())
		slog.Info("runDuplicateScanJob done", "job_id", jobID, "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Info("runDuplicateScanJob start", "job_id", jobID, "solution_id", solutionID)

	ctx := context.Background()
	_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "running", 0, 0, 0, nil, "", nil)

	sol, err := s.loadSolutionWithChanges(ctx, solutionID, sessionID)
	if err != nil {
		finished := time.Now()
		_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "failed", 0, 0, 0, nil, err.Error(), &finished)
		return
	}

	total := len(s.collectAllGraphicObjectRefs(sol))
	_ = s.store.UpdateComplianceDuplicateJobTotal(ctx, jobID, total)
	_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "running", 0, 0, 0, nil, "", nil)

	var processed atomic.Int64
	done := make(chan struct{})
	var progressWg sync.WaitGroup
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "running", int(processed.Load()), 0, 0, nil, "", nil)
			}
		}
	}()

	dupTagCount, violatedCount, err := s.runForcedDuplicateScanMemoryFirst(ctx, jobID, solutionID, sol, rule, func() {
		processed.Add(1)
	})

	close(done)
	progressWg.Wait()

	processedCount := int(processed.Load())
	if processedCount == 0 && total > 0 && err == nil {
		processedCount = total
	}

	if err != nil {
		finished := time.Now()
		_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "failed", processedCount, dupTagCount, violatedCount, nil, err.Error(), &finished)
		return
	}

	finished := time.Now()
	_ = s.store.UpdateComplianceDuplicateJobProgress(ctx, jobID, "completed", processedCount, dupTagCount, violatedCount, nil, "", &finished)
}

func (s *SolutionService) GetDuplicateScanJob(ctx context.Context, solutionID, jobID uuid.UUID) (repository.ComplianceDuplicateJob, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetDuplicateScanJob done", "job_id", jobID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	job, err := s.store.GetComplianceDuplicateJob(ctx, jobID)
	if err != nil {
		return repository.ComplianceDuplicateJob{}, err
	}
	if job.SolutionID != solutionID {
		err = fmt.Errorf("job not found")
		return repository.ComplianceDuplicateJob{}, err
	}
	if job.Status == "completed" && len(job.Result) == 0 {
		result, buildErr := s.buildDuplicateScanResultFromStore(ctx, solutionID, job.RuleID)
		if buildErr == nil {
			if resultJSON, marshalErr := json.Marshal(result); marshalErr == nil {
				job.Result = resultJSON
			}
		}
	}
	return job, nil
}

func (s *SolutionService) PatchDuplicateTag(ctx context.Context, sessionID uuid.UUID, solutionID uuid.UUID, req PatchDuplicateTagRequest) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("PatchDuplicateTag done", "solution_id", solutionID, "scope", req.Scope, "object_id", req.ObjectID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Info("PatchDuplicateTag start", "solution_id", solutionID, "scope", req.Scope, "object_id", req.ObjectID)

	newTag := strings.TrimSpace(req.NewTag)
	if req.SchemeXMLID == "" || req.ObjectID == "" {
		err = fmt.Errorf("scheme_xml_id and object_id required")
		return err
	}

	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return err
	}
	changes, err := s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return err
	}
	applyAllChanges(&sol, changes)
	drafts, err := s.store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		return err
	}
	applyAllChanges(&sol, drafts)

	switch req.Scope {
	case "command":
		err = s.patchCommandTag(ctx, sessionID, solutionID, req.SchemeXMLID, req, newTag, &sol)
	case "protection":
		err = s.patchProtectionTag(ctx, sessionID, solutionID, req.SchemeXMLID, req, newTag, &sol)
	default:
		err = s.patchGraphicObjectTag(ctx, sessionID, solutionID, req, newTag, &sol)
	}
	if err == nil {
		s.enqueueDirtyAfterTagChange(ctx, solutionID, req.ObjectID, "patch_duplicate_tag")
	}
	return err
}

func (s *SolutionService) findGraphicObjectInSolution(sol domain.Solution, schemeXMLID, objectID string) (*domain.GraphicObject, bool) {
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			if scheme.Id != schemeXMLID {
				continue
			}
			for i := range scheme.GraphicObject {
				if scheme.GraphicObject[i].Id == objectID {
					return &scheme.GraphicObject[i], true
				}
			}
		}
	}
	return nil, false
}

func (s *SolutionService) patchGraphicObjectTag(ctx context.Context, sessionID, solutionID uuid.UUID, req PatchDuplicateTagRequest, newTag string, sol *domain.Solution) error {
	obj, ok := s.findGraphicObjectInSolution(*sol, req.SchemeXMLID, req.ObjectID)
	if !ok {
		return fmt.Errorf("object not found")
	}
	updated := *obj
	if req.Scope == "property" {
		if len(updated.TechnologyObject) == 0 {
			return fmt.Errorf("technology object not found")
		}
		found := false
		for i := range updated.TechnologyObject[0].TechnologyProperty {
			if updated.TechnologyObject[0].TechnologyProperty[i].Id == req.PropertyID {
				updated.TechnologyObject[0].TechnologyProperty[i].Tag = newTag
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("property not found")
		}
	} else {
		updated.Tag = newTag
		if len(updated.TechnologyObject) > 0 {
			updated.TechnologyObject[0].Tag = newTag
		}
	}
	patch, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	return s.store.UpsertDraftChange(ctx, uuid.New(), sessionID, req.SchemeXMLID, "graphic_object", req.ObjectID, patch)
}

func (s *SolutionService) patchCommandTag(ctx context.Context, sessionID, solutionID uuid.UUID, schemeXMLID string, req PatchDuplicateTagRequest, newTag string, sol *domain.Solution) error {
	if req.EntityID == "" {
		return fmt.Errorf("entity_id required")
	}
	var updated domain.ControlCommand
	found := false
	for _, cmd := range sol.ControlCommands.ControlCommand {
		if cmd.Id == req.EntityID {
			updated = cmd
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("command not found")
	}
	if req.PropertyID != "" {
		propFound := false
		for i := range updated.TechnologyProperty {
			if updated.TechnologyProperty[i].Id == req.PropertyID {
				updated.TechnologyProperty[i].Tag = newTag
				propFound = true
				break
			}
		}
		if !propFound {
			return fmt.Errorf("command property not found")
		}
	} else {
		updated.Tag = newTag
	}
	patch, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	return s.store.UpsertDraftChange(ctx, uuid.New(), sessionID, schemeXMLID, "control_command", req.EntityID, patch)
}

func (s *SolutionService) patchProtectionTag(ctx context.Context, sessionID, solutionID uuid.UUID, schemeXMLID string, req PatchDuplicateTagRequest, newTag string, sol *domain.Solution) error {
	if req.EntityID == "" {
		return fmt.Errorf("entity_id required")
	}
	var updated domain.Automation
	found := false
	for _, auto := range sol.Automations.Automation {
		if auto.Id == req.EntityID {
			updated = auto
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("protection not found")
	}
	if req.PropertyID != "" {
		propFound := false
		for i := range updated.TechnologyProperty {
			if updated.TechnologyProperty[i].Id == req.PropertyID {
				updated.TechnologyProperty[i].Tag = newTag
				propFound = true
				break
			}
		}
		if !propFound {
			return fmt.Errorf("protection property not found")
		}
	} else {
		updated.Tag = newTag
	}
	patch, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	return s.store.UpsertDraftChange(ctx, uuid.New(), sessionID, schemeXMLID, "automation", req.EntityID, patch)
}
