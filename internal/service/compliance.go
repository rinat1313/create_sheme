package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

var ErrInvalidSessionID = errors.New("invalid session_id")

func isDuplicateTagRule(rule repository.ComplianceRule) bool {
	return rule.TextCondition == "duplicate_tag"
}

type ComplianceRuleInput struct {
	ID              string          `json:"id"`
	Template        string          `json:"template"`
	ParameterScope  string          `json:"parameter_scope"`
	EntityTemplate  string          `json:"entity_template"`
	Parameter       string          `json:"parameter"`
	ValueSource     string          `json:"value_source"`
	TextCondition   string          `json:"text_condition"`
	ExpectedText    string          `json:"expected_text"`
	EmptyAllowed    bool            `json:"empty_allowed"`
	Indication      json.RawMessage `json:"indication"`
}

func (s *SolutionService) ListComplianceRules(ctx context.Context) ([]repository.ComplianceRule, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListComplianceRules done", "duration_ms", time.Since(start).Milliseconds())
	}()
	return s.store.ListComplianceRules(ctx, UserIDFromContext(ctx), nil)
}

func (s *SolutionService) CreateComplianceRule(ctx context.Context, input ComplianceRuleInput) (repository.ComplianceRule, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("CreateComplianceRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	rule := repository.ComplianceRule{
		ID:             uuid.New(),
		UserID:         UserIDFromContext(ctx),
		Template:       input.Template,
		ParameterScope: normalizeParameterScope(input.ParameterScope),
		EntityTemplate: input.EntityTemplate,
		Parameter:      input.Parameter,
		ValueSource:    normalizeValueSource(input.ValueSource),
		TextCondition:  input.TextCondition,
		ExpectedText:   input.ExpectedText,
		EmptyAllowed:   input.EmptyAllowed,
		Indication:     input.Indication,
	}
	if input.TextCondition == "duplicate_tag" {
		count, countErr := s.countDuplicateTagRules(ctx, nil)
		if countErr != nil {
			err = countErr
			return rule, err
		}
		if count > 0 {
			err = ErrDuplicateRuleExists
			return rule, err
		}
		rule.Template = "*"
		rule.ParameterScope = "object"
		rule.EntityTemplate = ""
		rule.Parameter = ""
		rule.ValueSource = "field"
		rule.ExpectedText = ""
		rule.EmptyAllowed = true
	}
	if len(rule.Indication) == 0 {
		rule.Indication = json.RawMessage(`{"type":"solid","color":"#F85149"}`)
	}
	err = s.store.InsertComplianceRule(ctx, rule)
	if err == nil {
		s.BumpRulesGen(ctx, rule.UserID)
	}
	return rule, err
}

func (s *SolutionService) UpdateComplianceRule(ctx context.Context, input ComplianceRuleInput) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("UpdateComplianceRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	id, err := uuid.Parse(input.ID)
	if err != nil {
		return err
	}
	rule := repository.ComplianceRule{
		ID:             id,
		UserID:         UserIDFromContext(ctx),
		Template:       input.Template,
		ParameterScope: normalizeParameterScope(input.ParameterScope),
		EntityTemplate: input.EntityTemplate,
		Parameter:      input.Parameter,
		ValueSource:    normalizeValueSource(input.ValueSource),
		TextCondition:  input.TextCondition,
		ExpectedText:   input.ExpectedText,
		EmptyAllowed:   input.EmptyAllowed,
		Indication:     input.Indication,
	}
	if input.TextCondition == "duplicate_tag" {
		count, countErr := s.countDuplicateTagRules(ctx, &id)
		if countErr != nil {
			err = countErr
			return err
		}
		if count > 0 {
			err = ErrDuplicateRuleExists
			return err
		}
		rule.Template = "*"
		rule.ParameterScope = "object"
		rule.EntityTemplate = ""
		rule.Parameter = ""
		rule.ValueSource = "field"
		rule.ExpectedText = ""
		rule.EmptyAllowed = true
	}
	if len(rule.Indication) == 0 {
		rule.Indication = json.RawMessage(`{"type":"solid","color":"#F85149"}`)
	}
	err = s.store.UpdateComplianceRule(ctx, rule)
	if err == nil {
		s.BumpRulesGen(ctx, rule.UserID)
	}
	return err
}

func (s *SolutionService) DeleteComplianceRule(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("DeleteComplianceRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	userID := UserIDFromContext(ctx)
	err = s.store.DeleteComplianceRule(ctx, userID, id)
	if err == nil {
		s.BumpRulesGen(ctx, userID)
	}
	return err
}

type ExportObjectsRequest struct {
	Template    string   `json:"template"`
	Fields      []string `json:"fields"`
	Format      string   `json:"format"`
	ChangedOnly bool     `json:"changed_only"`
	SessionID   string   `json:"session_id"`
	TagPresence string   `json:"tag_presence"`
}

const (
	TagPresenceAll      = "all"
	TagPresenceNonEmpty = "non_empty"
	TagPresenceEmpty    = "empty"
)

var ErrInvalidTagPresence = errors.New("invalid tag_presence")

func normalizeTagPresence(v string) (string, error) {
	switch strings.TrimSpace(v) {
	case "", TagPresenceAll:
		return TagPresenceAll, nil
	case TagPresenceNonEmpty:
		return TagPresenceNonEmpty, nil
	case TagPresenceEmpty:
		return TagPresenceEmpty, nil
	default:
		return "", ErrInvalidTagPresence
	}
}

func normalizeValueSource(src string) string {
	if src == "tag" {
		return "tag"
	}
	return "field"
}

func normalizeParameterScope(scope string) string {
	switch scope {
	case "command", "protection", "property":
		return scope
	default:
		return "object"
	}
}

func (s *SolutionService) loadSolutionWithChanges(ctx context.Context, solutionID uuid.UUID, sessionID *uuid.UUID) (domain.Solution, error) {
	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return domain.Solution{}, err
	}
	changes, err := s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return domain.Solution{}, err
	}
	applyAllChanges(&sol, changes)
	if sessionID != nil {
		drafts, draftErr := s.store.ListDraftChanges(ctx, *sessionID)
		if draftErr != nil {
			return domain.Solution{}, draftErr
		}
		applyAllChanges(&sol, drafts)
	}
	return sol, nil
}

func (s *SolutionService) ListComplianceTargets(ctx context.Context, solutionID uuid.UUID, objectTemplate, scope string, sessionID *uuid.UUID) ([]string, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ListComplianceTargets done", "solution_id", solutionID, "object_template", objectTemplate, "scope", scope, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("ListComplianceTargets start", "solution_id", solutionID, "object_template", objectTemplate, "scope", scope)
	_ = sessionID

	scope = normalizeParameterScope(scope)
	if scope != "command" && scope != "protection" {
		err = fmt.Errorf("scope must be command or protection")
		return nil, err
	}

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return nil, err
	}
	var targets []string
	targets, err = s.store.GetTemplateEntities(ctx, solutionID, objectTemplate, scope)
	if err != nil {
		return nil, err
	}
	if targets == nil {
		targets = []string{}
	}
	return targets, nil
}

func listComplianceTargetsFromSolution(sol domain.Solution, objectTemplate, scope string) ([]string, error) {
	scope = normalizeParameterScope(scope)
	if scope != "command" && scope != "protection" {
		return nil, fmt.Errorf("scope must be command or protection")
	}

	targetKeys := buildTargetKeysForTemplate(sol, objectTemplate)
	seen := map[string]bool{}
	var result []string
	if scope == "command" {
		for _, cmd := range sol.ControlCommands.ControlCommand {
			if cmd.Template == "" || cmd.Target == "" || seen[cmd.Template] {
				continue
			}
			if _, ok := targetKeys[cmd.Target]; ok {
				seen[cmd.Template] = true
				result = append(result, cmd.Template)
			}
		}
	} else {
		for _, auto := range sol.Automations.Automation {
			if auto.Template == "" || auto.Target == "" || seen[auto.Template] {
				continue
			}
			if _, ok := targetKeys[auto.Target]; ok {
				seen[auto.Template] = true
				result = append(result, auto.Template)
			}
		}
	}
	sort.Strings(result)
	return result, nil
}

func (s *SolutionService) ListComplianceTargetParameters(ctx context.Context, solutionID uuid.UUID, scope, entityTemplate string, sessionID *uuid.UUID) ([]string, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ListComplianceTargetParameters done", "solution_id", solutionID, "scope", scope, "entity_template", entityTemplate, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("ListComplianceTargetParameters start", "solution_id", solutionID, "scope", scope, "entity_template", entityTemplate)
	_ = sessionID

	scope = normalizeParameterScope(scope)
	if scope != "command" && scope != "protection" {
		err = fmt.Errorf("scope must be command or protection")
		return nil, err
	}
	if entityTemplate == "" {
		err = fmt.Errorf("entity_template required")
		return nil, err
	}

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return nil, err
	}
	var params []string
	params, err = s.store.GetTemplateParameters(ctx, solutionID, scope, entityTemplate)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = []string{}
	}
	return params, nil
}

func listComplianceTargetParametersFromSolution(sol domain.Solution, scope, entityTemplate string) ([]string, error) {
	scope = normalizeParameterScope(scope)
	if scope != "command" && scope != "protection" {
		return nil, fmt.Errorf("scope must be command or protection")
	}
	if entityTemplate == "" {
		return nil, fmt.Errorf("entity_template required")
	}

	seen := map[string]bool{}
	for _, name := range baseComplianceParameters {
		seen[name] = true
	}
	var extra []string
	collectProps := func(props []domain.TechnologyProperty) {
		for _, p := range props {
			key := propertyKey(p)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			extra = append(extra, key)
		}
	}
	if scope == "command" {
		for _, cmd := range sol.ControlCommands.ControlCommand {
			if cmd.Template != entityTemplate {
				continue
			}
			collectProps(cmd.TechnologyProperty)
		}
	} else {
		for _, auto := range sol.Automations.Automation {
			if auto.Template != entityTemplate {
				continue
			}
			collectProps(auto.TechnologyProperty)
		}
	}
	sort.Strings(extra)
	return append(append([]string{}, baseComplianceParameters...), extra...), nil
}

func (s *SolutionService) ListTemplateParameters(ctx context.Context, solutionID uuid.UUID, template string, sessionID *uuid.UUID) ([]string, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ListTemplateParameters done", "solution_id", solutionID, "template", template, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("ListTemplateParameters start", "solution_id", solutionID, "template", template)
	_ = sessionID

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return nil, err
	}
	var params []string
	params, err = s.store.GetTemplateParameters(ctx, solutionID, "object", template)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = []string{}
	}
	return params, nil
}

func (s *SolutionService) ListTemplateCharacteristics(ctx context.Context, solutionID uuid.UUID, template string, sessionID *uuid.UUID) ([]string, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ListTemplateCharacteristics done", "solution_id", solutionID, "template", template, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("ListTemplateCharacteristics start", "solution_id", solutionID, "template", template)
	_ = sessionID

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return nil, err
	}
	var params []string
	params, err = s.store.GetTemplateParameters(ctx, solutionID, "property", template)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = []string{}
	}
	return params, nil
}

func listTemplateParametersFromSolution(sol domain.Solution, template string) []string {
	seen := map[string]bool{}
	for _, name := range baseComplianceParameters {
		seen[name] = true
	}
	var extra []string
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				if obj.TechnologyTemplate() != template {
					continue
				}
				if len(obj.TechnologyObject) == 0 {
					continue
				}
				for _, p := range obj.TechnologyObject[0].TechnologyProperty {
					key := propertyKey(p)
					if key == "" || seen[key] {
						continue
					}
					seen[key] = true
					extra = append(extra, key)
				}
			}
		}
	}
	sort.Strings(extra)
	return append(append([]string{}, baseComplianceParameters...), extra...)
}

func listTemplateCharacteristicsFromSolution(sol domain.Solution, template string) []string {
	seen := map[string]bool{}
	var result []string
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				if obj.TechnologyTemplate() != template {
					continue
				}
				if len(obj.TechnologyObject) == 0 {
					continue
				}
				for _, p := range obj.TechnologyObject[0].TechnologyProperty {
					key := propertyKey(p)
					if key == "" || seen[key] {
						continue
					}
					seen[key] = true
					result = append(result, key)
				}
			}
		}
	}
	sort.Strings(result)
	return result
}

func (s *SolutionService) collectChangedObjectIDs(ctx context.Context, solutionID uuid.UUID, sessionID *uuid.UUID) (map[string]bool, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("collectChangedObjectIDs done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("collectChangedObjectIDs start", "solution_id", solutionID)

	ids := map[string]bool{}
	var durable []repository.SolutionChangedObjectRow
	durable, err = s.store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	for _, row := range durable {
		if row.EntityType == "graphic_object" {
			ids[row.EntityXMLID] = true
		}
	}
	var changes []repository.ChangeRow
	changes, err = s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	for _, ch := range changes {
		if ch.EntityType == "graphic_object" {
			ids[ch.EntityXMLID] = true
		}
	}
	if sessionID != nil {
		var drafts []repository.ChangeRow
		drafts, err = s.store.ListDraftChanges(ctx, *sessionID)
		if err != nil {
			return nil, err
		}
		for _, ch := range drafts {
			if ch.EntityType == "graphic_object" {
				ids[ch.EntityXMLID] = true
			}
		}
	}

	var acks []repository.ExportChangeAckRow
	acks, err = s.store.ListExportChangeAcks(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	subtractAckedObjectIDs(ids, acks)
	return ids, nil
}

func subtractAckedObjectIDs(ids map[string]bool, acks []repository.ExportChangeAckRow) {
	for _, ack := range acks {
		if ack.EntityType == "graphic_object" {
			delete(ids, ack.EntityXMLID)
		}
	}
}

func (s *SolutionService) ExportObjects(ctx context.Context, solutionID uuid.UUID, req ExportObjectsRequest) ([]byte, string, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ExportObjects done", "solution_id", solutionID, "template", req.Template, "tag_presence", req.TagPresence, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	req.TagPresence, err = normalizeTagPresence(req.TagPresence)
	if err != nil {
		return nil, "", err
	}

	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return nil, "", err
	}
	changes, err := s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return nil, "", err
	}
	applyAllChanges(&sol, changes)

	var sessionID *uuid.UUID
	if req.SessionID != "" {
		sid, parseErr := uuid.Parse(req.SessionID)
		if parseErr != nil {
			err = ErrInvalidSessionID
			return nil, "", err
		}
		sessionID = &sid
		drafts, draftErr := s.store.ListDraftChanges(ctx, sid)
		if draftErr != nil {
			return nil, "", draftErr
		}
		applyAllChanges(&sol, drafts)
	}

	var changedIDs map[string]bool
	if req.ChangedOnly {
		changedIDs, err = s.collectChangedObjectIDs(ctx, solutionID, sessionID)
		if err != nil {
			return nil, "", err
		}
	}

	rows := buildExportRows(sol, req, changedIDs)

	format := strings.ToLower(req.Format)
	if format == "" {
		format = "csv"
	}
	if format == "json" {
		data, marshalErr := marshalOrderedJSONRows(req.Fields, rows)
		return data, "objects.json", marshalErr
	}
	return buildCSV(req.Fields, rows), "objects.csv", nil
}

func matchesTagPresence(tag, presence string) bool {
	switch presence {
	case TagPresenceNonEmpty:
		return strings.TrimSpace(tag) != ""
	case TagPresenceEmpty:
		return strings.TrimSpace(tag) == ""
	default:
		return true
	}
}

func buildExportRows(sol domain.Solution, req ExportObjectsRequest, changedIDs map[string]bool) []map[string]string {
	start := time.Now()
	defer func() {
		slog.Debug("buildExportRows done",
			"template", req.Template,
			"changed_only", req.ChangedOnly,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("buildExportRows start", "template", req.Template, "changed_only", req.ChangedOnly)

	presence := req.TagPresence
	if presence == "" {
		presence = TagPresenceAll
	}
	var rows []map[string]string
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				template := obj.TechnologyTemplate()
				if req.Template != "" && template != req.Template {
					continue
				}
				if req.ChangedOnly && !changedIDs[obj.Id] {
					continue
				}
				var to *domain.TechnologyObject
				if len(obj.TechnologyObject) > 0 {
					to = &obj.TechnologyObject[0]
				}
				objectTag := resolveObjectField("Tag", &obj, to)
				if !matchesTagPresence(objectTag, presence) {
					continue
				}
				row := make(map[string]string)
				for _, field := range req.Fields {
					row[field] = resolveObjectField(field, &obj, to)
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func isObjectScalarParameter(key string) bool {
	start := time.Now()
	defer func() {
		slog.Debug("isObjectScalarParameter done", "key", key, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("isObjectScalarParameter start", "key", key)
	switch key {
	case "Tag", "Name", "Description", "Id", "ExternalCode":
		return true
	default:
		return false
	}
}

func findTechnologyProperty(to *domain.TechnologyObject, key string) (domain.TechnologyProperty, bool) {
	start := time.Now()
	defer func() {
		slog.Debug("findTechnologyProperty done", "key", key, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("findTechnologyProperty start", "key", key)
	if to == nil {
		return domain.TechnologyProperty{}, false
	}
	for _, p := range to.TechnologyProperty {
		if p.Name == key || p.Template == key {
			return p, true
		}
	}
	return domain.TechnologyProperty{}, false
}

func resolvePropertyTag(to *domain.TechnologyObject, key string) string {
	start := time.Now()
	var result string
	defer func() {
		slog.Debug("resolvePropertyTag done", "key", key, "result", result, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("resolvePropertyTag start", "key", key)
	if p, ok := findTechnologyProperty(to, key); ok {
		result = p.Tag
	}
	return result
}

func resolveObjectField(field string, obj *domain.GraphicObject, to *domain.TechnologyObject) string {
	switch field {
	case "Tag":
		if to != nil && to.Tag != "" {
			return to.Tag
		}
		return obj.Tag
	case "Name":
		if to != nil && to.Name != "" {
			return to.Name
		}
		return obj.Name
	case "Description":
		if to != nil && to.Description != "" {
			return to.Description
		}
		return obj.Description
	case "Id":
		if to != nil {
			return strings.TrimSpace(to.Id)
		}
		return ""
	case "ExternalCode":
		if to != nil && to.ExternalCode != "" {
			return to.ExternalCode
		}
		return obj.ExternalCode
	case "Template":
		if to != nil {
			return to.Template
		}
		return ""
	default:
		if p, ok := findTechnologyProperty(to, field); ok {
			return technologyPropertyValue(p)
		}
		return ""
	}
}

func technologyPropertyValue(p domain.TechnologyProperty) string {
	if p.RealValue != 0 {
		return fmt.Sprintf("%v", p.RealValue)
	}
	if p.StringValue != "" {
		return p.StringValue
	}
	if p.IntegerValue != 0 {
		return fmt.Sprintf("%d", p.IntegerValue)
	}
	if p.BooleanValue != "" {
		return p.BooleanValue
	}
	if p.PointsValue != "" {
		return p.PointsValue
	}
	return ""
}

func (s *SolutionService) CountChangedObjects(ctx context.Context, solutionID uuid.UUID, sessionID *uuid.UUID) (int, error) {
	start := time.Now()
	var err error
	var count int
	defer func() {
		slog.Info("CountChangedObjects done", "solution_id", solutionID, "count", count, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("CountChangedObjects start", "solution_id", solutionID)
	var ids map[string]bool
	ids, err = s.collectChangedObjectIDs(ctx, solutionID, sessionID)
	if err != nil {
		return 0, err
	}
	count = len(ids)
	return count, nil
}

func (s *SolutionService) AckChangedObjects(ctx context.Context, solutionID uuid.UUID, sessionID *uuid.UUID) (int, error) {
	start := time.Now()
	var err error
	var count int
	defer func() {
		slog.Info("AckChangedObjects done", "solution_id", solutionID, "count", count, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("AckChangedObjects start", "solution_id", solutionID)

	rawIDs := map[string]bool{}
	var durable []repository.SolutionChangedObjectRow
	durable, err = s.store.ListSolutionChangedObjects(ctx, solutionID)
	if err != nil {
		return 0, err
	}
	for _, row := range durable {
		if row.EntityType == "graphic_object" {
			rawIDs[row.EntityXMLID] = true
		}
	}
	var changes []repository.ChangeRow
	changes, err = s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return 0, err
	}
	for _, ch := range changes {
		if ch.EntityType == "graphic_object" {
			rawIDs[ch.EntityXMLID] = true
		}
	}
	if sessionID != nil {
		var drafts []repository.ChangeRow
		drafts, err = s.store.ListDraftChanges(ctx, *sessionID)
		if err != nil {
			return 0, err
		}
		for _, ch := range drafts {
			if ch.EntityType == "graphic_object" {
				rawIDs[ch.EntityXMLID] = true
			}
		}
	}

	entityXMLIDs := make([]string, 0, len(rawIDs))
	for id := range rawIDs {
		entityXMLIDs = append(entityXMLIDs, id)
	}
	sort.Strings(entityXMLIDs)

	if err = s.store.AckExportChangedObjects(ctx, solutionID, "graphic_object", entityXMLIDs); err != nil {
		return 0, err
	}
	if err = s.store.DeleteSolutionChangedObjects(ctx, solutionID, "graphic_object", entityXMLIDs); err != nil {
		return 0, err
	}

	count, err = s.CountChangedObjects(ctx, solutionID, sessionID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func buildCSV(fields []string, rows []map[string]string) []byte {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write(fields)
	for _, row := range rows {
		record := make([]string, len(fields))
		for i, f := range fields {
			record[i] = row[f]
		}
		_ = w.Write(record)
	}
	w.Flush()
	return []byte(buf.String())
}

func marshalOrderedJSONRows(fields []string, rows []map[string]string) ([]byte, error) {
	ordered := make([]json.RawMessage, len(rows))
	for i, row := range rows {
		var buf strings.Builder
		buf.WriteByte('{')
		for j, field := range fields {
			if j > 0 {
				buf.WriteByte(',')
			}
			keyJSON, err := json.Marshal(field)
			if err != nil {
				return nil, err
			}
			valJSON, err := json.Marshal(row[field])
			if err != nil {
				return nil, err
			}
			buf.Write(keyJSON)
			buf.WriteByte(':')
			buf.Write(valJSON)
		}
		buf.WriteByte('}')
		ordered[i] = json.RawMessage(buf.String())
	}
	return json.MarshalIndent(ordered, "", "  ")
}
