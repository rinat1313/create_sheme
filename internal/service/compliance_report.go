package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"create_sheme/internal/cache"
	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

type ComplianceIndicator struct {
	RuleID string `json:"rule_id"`
	Color  string `json:"color"`
	Type   string `json:"type"`
}

type ComplianceNodeHint struct {
	XMLID      string                `json:"xml_id"`
	Type       string                `json:"type"`
	Name       string                `json:"name"`
	Indicators []ComplianceIndicator `json:"indicators"`
}

type ComplianceReport struct {
	Nodes []ComplianceNodeHint `json:"nodes"`
}

type ComplianceReportOptions struct {
	ProjectXMLID string
	SchemeXMLID  string
}

func (s *SolutionService) BuildComplianceReport(ctx context.Context, solutionID uuid.UUID, opts ComplianceReportOptions) (ComplianceReport, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("BuildComplianceReport done", "solution_id", solutionID, "project_xml_id", opts.ProjectXMLID, "scheme_xml_id", opts.SchemeXMLID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	userID := UserIDFromContext(ctx)
	rulesGen := s.getRulesGen(ctx, userID)
	solGen := s.getSolutionGen(ctx, solutionID)
	scope := cache.ComplianceScope(opts.ProjectXMLID, opts.SchemeXMLID)
	key := cache.ComplianceKey(userID, rulesGen, solutionID, solGen, scope)
	if raw, hit := s.cacheGet(ctx, key, "compliance", solutionID); hit {
		var cached ComplianceReport
		if unmarshalErr := json.Unmarshal(raw, &cached); unmarshalErr == nil {
			return cached, nil
		}
	}

	report, buildErr := s.buildComplianceReportUncached(ctx, solutionID, opts)
	if buildErr != nil {
		err = buildErr
		return ComplianceReport{}, err
	}
	if raw, marshalErr := json.Marshal(report); marshalErr == nil {
		s.cacheSet(ctx, key, raw, s.cacheCfg.ComplianceTTL, "compliance", solutionID)
	}
	return report, nil
}

func (s *SolutionService) buildComplianceReportUncached(ctx context.Context, solutionID uuid.UUID, opts ComplianceReportOptions) (ComplianceReport, error) {
	allRules, err := s.store.ListComplianceRules(ctx, UserIDFromContext(ctx), &solutionID)
	if err != nil {
		return ComplianceReport{}, err
	}
	var rules []repository.ComplianceRule
	for _, r := range allRules {
		if !isDuplicateTagRule(r) {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 {
		return ComplianceReport{Nodes: []ComplianceNodeHint{}}, nil
	}

	meta, err := s.loadSolutionMetadata(ctx, solutionID)
	if err != nil {
		return ComplianceReport{}, err
	}
	changes, err := s.store.ListMetadataWorkingChanges(ctx, solutionID)
	if err != nil {
		return ComplianceReport{}, err
	}
	applyMetadataChanges(&meta, changes)
	sol := meta.asSolution()

	var schemes []repository.SchemeRow
	if opts.SchemeXMLID != "" {
		sch, getErr := s.store.GetScheme(ctx, solutionID, opts.SchemeXMLID)
		if getErr != nil {
			return ComplianceReport{}, getErr
		}
		schemes = []repository.SchemeRow{sch}
	} else if opts.ProjectXMLID != "" {
		schemes, err = s.store.ListSchemesByProject(ctx, solutionID, opts.ProjectXMLID)
		if err != nil {
			return ComplianceReport{}, err
		}
	} else {
		schemes, err = s.store.ListSchemes(ctx, solutionID)
		if err != nil {
			return ComplianceReport{}, err
		}
	}

	nodes, err := s.store.ListNodes(ctx, solutionID)
	if err != nil {
		return ComplianceReport{}, err
	}

	nodeByXML := map[string]repository.NodeRow{}
	for _, n := range nodes {
		nodeByXML[n.XMLID] = n
	}

	schemeIndicators := map[string][]ComplianceIndicator{}
	projectRuleSets := map[string]map[string]ComplianceIndicator{}
	folderRuleSets := map[string]map[string]ComplianceIndicator{}

	for _, sch := range schemes {
		var scheme domain.GraphicScheme
		if err := json.Unmarshal(sch.Payload, &scheme); err != nil {
			continue
		}
		schemeChanges, _ := s.store.ListWorkingChangesForScheme(ctx, solutionID, sch.XMLID)
		applyChangesToScheme(&scheme, schemeChanges)
		ruleSet := map[string]ComplianceIndicator{}
		for _, obj := range scheme.GraphicObject {
			for _, ind := range evaluateObjectComplianceAll(sol, obj, rules) {
				ruleSet[ind.RuleID] = ind
			}
		}
		if len(ruleSet) == 0 {
			continue
		}
		indicators := mapValues(ruleSet)
		schemeIndicators[sch.XMLID] = indicators

		if sch.ProjectXMLID != "" {
			if projectRuleSets[sch.ProjectXMLID] == nil {
				projectRuleSets[sch.ProjectXMLID] = map[string]ComplianceIndicator{}
			}
			for id, ind := range ruleSet {
				projectRuleSets[sch.ProjectXMLID][id] = ind
			}
			if n, ok := nodeByXML[sch.ProjectXMLID]; ok && n.ParentID != nil {
				parent, ok := findNodeByID(nodes, *n.ParentID)
				if ok && parent.NodeType == "folder" {
					if folderRuleSets[parent.XMLID] == nil {
						folderRuleSets[parent.XMLID] = map[string]ComplianceIndicator{}
					}
					for id, ind := range ruleSet {
						folderRuleSets[parent.XMLID][id] = ind
					}
				}
			}
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

	return ComplianceReport{Nodes: hints}, nil
}

func mapValues(m map[string]ComplianceIndicator) []ComplianceIndicator {
	result := make([]ComplianceIndicator, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func findNodeByID(nodes []repository.NodeRow, id uuid.UUID) (repository.NodeRow, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
	}
	return repository.NodeRow{}, false
}

func evaluateObjectComplianceAll(sol domain.Solution, obj domain.GraphicObject, rules []repository.ComplianceRule) []ComplianceIndicator {
	template := obj.TechnologyTemplate()
	var matched []repository.ComplianceRule
	for _, r := range rules {
		if isDuplicateTagRule(r) {
			continue
		}
		if r.Template == template {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	var to *domain.TechnologyObject
	if len(obj.TechnologyObject) > 0 {
		to = &obj.TechnologyObject[0]
	}
	var result []ComplianceIndicator
	for _, rule := range matched {
		val := resolveComplianceValues(rule, sol, &obj, to)
		isEmpty := strings.TrimSpace(val) == ""
		violated := false
		if isEmpty {
			if !rule.EmptyAllowed {
				violated = true
			}
		} else {
			expected := rule.ExpectedText
			if rule.TextCondition == "must_not_contain" && strings.Contains(val, expected) {
				violated = true
			}
			if rule.TextCondition == "must_contain" && expected != "" && !strings.Contains(val, expected) {
				violated = true
			}
			if rule.TextCondition == "exact_match" && val != expected {
				violated = true
			}
			if rule.TextCondition == "must_not_exact" && val == expected {
				violated = true
			}
		}
		if violated {
			ind := parseIndicationJSON(rule.Indication)
			result = append(result, ComplianceIndicator{
				RuleID: rule.ID.String(),
				Color:  indicationColor(ind),
				Type:   indicationType(ind),
			})
		}
	}
	return result
}

func indicationType(ind map[string]any) string {
	if t, ok := ind["type"].(string); ok && t != "" {
		return t
	}
	return "solid"
}

func resolveComplianceValues(rule repository.ComplianceRule, sol domain.Solution, obj *domain.GraphicObject, to *domain.TechnologyObject) string {
	scope := rule.ParameterScope
	if scope == "" {
		scope = "object"
	}
	switch scope {
	case "property":
		return resolvePropertyComplianceField(rule, to)
	case "command":
		for _, cmd := range sol.ControlCommands.ControlCommand {
			if cmd.Template != rule.EntityTemplate {
				continue
			}
			if matchesTarget(cmd.Target, *obj) {
				return resolveEntityComplianceField(rule, cmd.Tag, cmd.Name, cmd.Description, cmd.Id, cmd.ExternalCode, cmd.TechnologyProperty)
			}
		}
		return ""
	case "protection":
		for _, auto := range sol.Automations.Automation {
			if auto.Template != rule.EntityTemplate {
				continue
			}
			if matchesTarget(auto.Target, *obj) {
				return resolveEntityComplianceField(rule, auto.Tag, auto.Name, auto.Description, auto.Id, auto.ExternalCode, auto.TechnologyProperty)
			}
		}
		return ""
	default:
		return resolveObjectComplianceField(rule, obj, to)
	}
}

func resolvePropertyComplianceField(rule repository.ComplianceRule, to *domain.TechnologyObject) string {
	if to == nil {
		return ""
	}
	switch rule.ValueSource {
	case "tag":
		return resolvePropertyTag(to, rule.Parameter)
	default:
		if p, ok := findTechnologyProperty(to, rule.Parameter); ok {
			return technologyPropertyValue(p)
		}
		return ""
	}
}

func resolveObjectComplianceField(rule repository.ComplianceRule, obj *domain.GraphicObject, to *domain.TechnologyObject) string {
	switch rule.ValueSource {
	case "tag":
		if isObjectScalarParameter(rule.Parameter) {
			return resolveObjectField(rule.Parameter, obj, to)
		}
		return resolvePropertyTag(to, rule.Parameter)
	default:
		return resolveObjectField(rule.Parameter, obj, to)
	}
}

func resolveEntityComplianceField(rule repository.ComplianceRule, tag, name, description, id, externalCode string, props []domain.TechnologyProperty) string {
	switch rule.ValueSource {
	case "tag":
		if isObjectScalarParameter(rule.Parameter) {
			return resolveEntityScalarField(rule.Parameter, tag, name, description, id, externalCode)
		}
		return resolveEntityPropertyTag(props, rule.Parameter)
	default:
		if isObjectScalarParameter(rule.Parameter) {
			return resolveEntityScalarField(rule.Parameter, tag, name, description, id, externalCode)
		}
		for _, p := range props {
			if p.Name == rule.Parameter || p.Template == rule.Parameter {
				return technologyPropertyValue(p)
			}
		}
		return ""
	}
}

func resolveEntityScalarField(field, tag, name, description, id, externalCode string) string {
	switch field {
	case "Tag":
		return tag
	case "Name":
		return name
	case "Description":
		return description
	case "Id":
		return id
	case "ExternalCode":
		return externalCode
	default:
		return ""
	}
}

func resolveEntityPropertyTag(props []domain.TechnologyProperty, key string) string {
	for _, p := range props {
		if p.Name == key || p.Template == key {
			return p.Tag
		}
	}
	return ""
}

func resolveComplianceField(rule repository.ComplianceRule, obj *domain.GraphicObject, to *domain.TechnologyObject) string {
	return resolveObjectComplianceField(rule, obj, to)
}

func parseIndicationJSON(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{"type": "solid", "color": "#F85149"}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{"type": "solid", "color": "#F85149"}
	}
	return m
}

func indicationColor(ind map[string]any) string {
	if c, ok := ind["color"].(string); ok && c != "" {
		return c
	}
	return "#F85149"
}
