package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrTagEditJobActive      = errors.New("tag edit job already running for this solution")
	ErrTagEditNoRules        = errors.New("no tag edit rules configured")
	ErrTagEditEmptyPattern   = errors.New("pattern must not be empty")
	ErrTagEditEmptyTemplate  = errors.New("object type must not be empty")
	ErrTagEditNoActions      = errors.New("at least one action is required")
	ErrTagEditInvalidImport  = errors.New("invalid tag edit rules file")
	ErrTagEditImportTooLarge = errors.New("tag edit rules file is too large")
)

const tagEditImportMaxBytes = 512 * 1024

const TagEditImportReadLimit = tagEditImportMaxBytes + 1

var tagEditJobLocks sync.Map

func tagEditMarksSolutionDirty(changed int) bool {
	return changed > 0
}

type TagEditActionInput struct {
	ParameterScope string `json:"parameter_scope"`
	EntityTemplate string `json:"entity_template"`
	Parameter      string `json:"parameter"`
	ValueSource    string `json:"value_source"`
	Action         string `json:"action"`
	Replacement    string `json:"replacement"`
	ReplaceMode    string `json:"replace_mode"`
}

type TagEditRuleInput struct {
	ID               string               `json:"id"`
	TextCondition    string               `json:"text_condition"`
	Pattern          string               `json:"pattern"`
	Action           string               `json:"action"`
	Replacement      string               `json:"replacement"`
	Template         string               `json:"template"`
	ParameterScope   string               `json:"parameter_scope"`
	EntityTemplate   string               `json:"entity_template"`
	Parameter        string               `json:"parameter"`
	ValueSource      string               `json:"value_source"`
	Actions          []TagEditActionInput `json:"actions"`
	EmptyAllowed     bool                 `json:"empty_allowed"`
	CheckProperties  bool                 `json:"check_properties"`
	CheckCommands    bool                 `json:"check_commands"`
	CheckProtections bool                 `json:"check_protections"`
	SortOrder        int                  `json:"sort_order"`
}

type TagEditPreview struct {
	TotalObjects   int `json:"total_objects"`
	ChangedObjects int `json:"changed_objects"`
}

type tagEditRulesExport struct {
	Version int                      `json:"version"`
	Kind    string                   `json:"kind"`
	Rules   []tagEditRuleExportItem  `json:"rules"`
}

type tagEditRuleExportItem struct {
	Template         string                    `json:"template"`
	ParameterScope   string                    `json:"parameter_scope"`
	EntityTemplate   string                    `json:"entity_template"`
	Parameter        string                    `json:"parameter"`
	ValueSource      string                    `json:"value_source"`
	TextCondition    string                    `json:"text_condition"`
	Pattern          string                    `json:"pattern"`
	CheckProperties  bool                      `json:"check_properties"`
	CheckCommands    bool                      `json:"check_commands"`
	CheckProtections bool                      `json:"check_protections"`
	Actions          []repository.TagEditAction `json:"actions"`
}

func (s *SolutionService) ListTagEditRules(ctx context.Context) ([]repository.TagEditRule, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListTagEditRules done", "duration_ms", time.Since(start).Milliseconds())
	}()
	return s.store.ListTagEditRules(ctx, UserIDFromContext(ctx))
}

func (s *SolutionService) CreateTagEditRule(ctx context.Context, input TagEditRuleInput) (repository.TagEditRule, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("CreateTagEditRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	rule, err := buildTagEditRule(uuid.New(), UserIDFromContext(ctx), input)
	if err != nil {
		return rule, err
	}
	err = s.store.InsertTagEditRule(ctx, rule)
	return rule, err
}

func (s *SolutionService) UpdateTagEditRule(ctx context.Context, input TagEditRuleInput) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("UpdateTagEditRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	id, err := uuid.Parse(input.ID)
	if err != nil {
		return err
	}
	rule, buildErr := buildTagEditRule(id, UserIDFromContext(ctx), input)
	if buildErr != nil {
		err = buildErr
		return err
	}
	err = s.store.UpdateTagEditRule(ctx, rule)
	return err
}

func (s *SolutionService) DeleteTagEditRule(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("DeleteTagEditRule done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	err = s.store.DeleteTagEditRule(ctx, UserIDFromContext(ctx), id)
	return err
}

func (s *SolutionService) ExportTagEditRules(ctx context.Context) ([]byte, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ExportTagEditRules done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	rules, err := s.store.ListTagEditRules(ctx, UserIDFromContext(ctx))
	if err != nil {
		return nil, err
	}
	out := tagEditRulesExport{Version: 1, Kind: "tag-edit-rules"}
	for _, rule := range rules {
		ensureTagEditActions(&rule)
		out.Rules = append(out.Rules, tagEditRuleExportItem{
			Template:         rule.Template,
			ParameterScope:   rule.ParameterScope,
			EntityTemplate:   rule.EntityTemplate,
			Parameter:        rule.Parameter,
			ValueSource:      rule.ValueSource,
			TextCondition:    rule.TextCondition,
			Pattern:          rule.Pattern,
			CheckProperties:  rule.CheckProperties,
			CheckCommands:    rule.CheckCommands,
			CheckProtections: rule.CheckProtections,
			Actions:          rule.Actions,
		})
	}
	if out.Rules == nil {
		out.Rules = []tagEditRuleExportItem{}
	}
	return json.MarshalIndent(out, "", "  ")
}

func (s *SolutionService) ImportTagEditRules(ctx context.Context, raw []byte) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("ImportTagEditRules done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	if len(raw) > tagEditImportMaxBytes {
		err = ErrTagEditImportTooLarge
		return err
	}
	var payload tagEditRulesExport
	if err = json.Unmarshal(raw, &payload); err != nil {
		err = ErrTagEditInvalidImport
		return err
	}
	if payload.Kind != "" && payload.Kind != "tag-edit-rules" {
		err = ErrTagEditInvalidImport
		return err
	}
	userID := UserIDFromContext(ctx)
	built := make([]repository.TagEditRule, 0, len(payload.Rules))
	for i, item := range payload.Rules {
		actions := make([]TagEditActionInput, 0, len(item.Actions))
		for _, act := range item.Actions {
			actions = append(actions, TagEditActionInput{
				ParameterScope: act.ParameterScope,
				EntityTemplate: act.EntityTemplate,
				Parameter:      act.Parameter,
				ValueSource:    act.ValueSource,
				Action:         act.Action,
				Replacement:    act.Replacement,
				ReplaceMode:    act.ReplaceMode,
			})
		}
		rule, buildErr := buildTagEditRule(uuid.New(), userID, TagEditRuleInput{
			TextCondition:    item.TextCondition,
			Pattern:          item.Pattern,
			Template:         item.Template,
			ParameterScope:   item.ParameterScope,
			EntityTemplate:   item.EntityTemplate,
			Parameter:        item.Parameter,
			ValueSource:      item.ValueSource,
			Actions:          actions,
			CheckProperties:  item.CheckProperties,
			CheckCommands:    item.CheckCommands,
			CheckProtections: item.CheckProtections,
			SortOrder:        i,
		})
		if buildErr != nil {
			err = buildErr
			return err
		}
		built = append(built, rule)
	}
	if err = s.store.DeleteTagEditRulesForUser(ctx, userID); err != nil {
		return err
	}
	for _, rule := range built {
		if err = s.store.InsertTagEditRule(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTagTextCondition(cond string) string {
	switch cond {
	case "starts_with", "ends_with", "empty":
		return cond
	default:
		return "contains"
	}
}

func normalizeTagAction(action string) string {
	if action == "replace" {
		return "replace"
	}
	return "delete"
}

func normalizeReplaceMode(mode string) string {
	switch mode {
	case "whole", "first", "all", "last":
		return mode
	default:
		return "found"
	}
}

func matchTagCondition(textCondition, pattern, tag string) bool {
	if textCondition == "empty" {
		return isTagEmpty(tag)
	}
	if pattern == "" {
		return false
	}
	start := time.Now()
	var matched bool
	defer func() {
		slog.Debug("matchTagCondition done", "text_condition", textCondition, "pattern", pattern, "matched", matched, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("matchTagCondition start", "text_condition", textCondition, "pattern", pattern)
	switch textCondition {
	case "starts_with":
		matched = strings.HasPrefix(tag, pattern)
	case "ends_with":
		matched = strings.HasSuffix(tag, pattern)
	default:
		matched = strings.Contains(tag, pattern)
	}
	return matched
}

func buildTagEditActions(input TagEditRuleInput) []repository.TagEditAction {
	if len(input.Actions) == 0 && (input.Action != "" || input.Replacement != "") {
		return []repository.TagEditAction{{
			ParameterScope: "object",
			Parameter:      "Tag",
			ValueSource:    "field",
			Action:         normalizeTagAction(input.Action),
			Replacement:    input.Replacement,
			ReplaceMode:    "found",
		}}
	}
	out := make([]repository.TagEditAction, 0, len(input.Actions))
	for _, act := range input.Actions {
		item := repository.TagEditAction{
			ParameterScope: normalizeParameterScope(act.ParameterScope),
			EntityTemplate: strings.TrimSpace(act.EntityTemplate),
			Parameter:      strings.TrimSpace(act.Parameter),
			ValueSource:    normalizeValueSource(act.ValueSource),
			Action:         normalizeTagAction(act.Action),
			Replacement:    act.Replacement,
			ReplaceMode:    normalizeReplaceMode(act.ReplaceMode),
		}
		if item.Parameter == "" {
			item.Parameter = "Tag"
		}
		out = append(out, item)
	}
	return out
}

func buildTagEditRule(id, userID uuid.UUID, input TagEditRuleInput) (repository.TagEditRule, error) {
	actions := buildTagEditActions(input)
	firstAction, firstReplacement := "delete", ""
	if len(actions) > 0 {
		firstAction = actions[0].Action
		firstReplacement = actions[0].Replacement
	}
	rule := repository.TagEditRule{
		ID:               id,
		UserID:           userID,
		TextCondition:    normalizeTagTextCondition(input.TextCondition),
		Pattern:          strings.TrimSpace(input.Pattern),
		Action:           firstAction,
		Replacement:      firstReplacement,
		Template:         strings.TrimSpace(input.Template),
		ParameterScope:   normalizeParameterScope(input.ParameterScope),
		EntityTemplate:   strings.TrimSpace(input.EntityTemplate),
		Parameter:        strings.TrimSpace(input.Parameter),
		ValueSource:      normalizeValueSource(input.ValueSource),
		Actions:          actions,
		CheckProperties:  input.CheckProperties,
		CheckCommands:    input.CheckCommands,
		CheckProtections: input.CheckProtections,
		SortOrder:        input.SortOrder,
	}
	if rule.Parameter == "" {
		rule.Parameter = "Tag"
	}
	if rule.Template == "" {
		return rule, ErrTagEditEmptyTemplate
	}
	if rule.TextCondition != "empty" && rule.Pattern == "" {
		return rule, ErrTagEditEmptyPattern
	}
	if rule.TextCondition == "empty" {
		rule.Pattern = ""
	}
	if len(rule.Actions) == 0 {
		return rule, ErrTagEditNoActions
	}
	return rule, nil
}

func ensureTagEditActions(rule *repository.TagEditRule) {
	if len(rule.Actions) > 0 {
		return
	}
	if rule.Action == "" && rule.Replacement == "" {
		return
	}
	rule.Actions = []repository.TagEditAction{{
		ParameterScope: "object",
		Parameter:      "Tag",
		ValueSource:    "field",
		Action:         normalizeTagAction(rule.Action),
		Replacement:    rule.Replacement,
		ReplaceMode:    "found",
	}}
}

func isTagEmpty(tag string) bool {
	return strings.TrimSpace(tag) == ""
}

func applySubstringReplace(textCondition, pattern, replacement, tag string) string {
	if pattern == "" {
		return tag
	}
	switch textCondition {
	case "starts_with":
		if strings.HasPrefix(tag, pattern) {
			return replacement + tag[len(pattern):]
		}
	case "ends_with":
		if strings.HasSuffix(tag, pattern) {
			return tag[:len(tag)-len(pattern)] + replacement
		}
	default:
		return strings.ReplaceAll(tag, pattern, replacement)
	}
	return tag
}

func tagEditEntitiesJSON(v any) [][]byte {
	raw, _ := json.Marshal(v)
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return nil
	}
	out := make([][]byte, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func commandJSONChanged(original [][]byte, index int, current any) bool {
	cur, _ := json.Marshal(current)
	if index < 0 || index >= len(original) {
		return true
	}
	return string(original[index]) != string(cur)
}

func applyReplaceMode(mode, textCondition, pattern, replacement, value string) string {
	if textCondition == "empty" || mode == "whole" {
		return replacement
	}
	if pattern == "" {
		return value
	}
	switch mode {
	case "first":
		if i := strings.Index(value, pattern); i >= 0 {
			return value[:i] + replacement + value[i+len(pattern):]
		}
	case "last":
		if i := strings.LastIndex(value, pattern); i >= 0 {
			return value[:i] + replacement + value[i+len(pattern):]
		}
	case "all":
		return strings.ReplaceAll(value, pattern, replacement)
	default:
		return applySubstringReplace(textCondition, pattern, replacement, value)
	}
	return value
}

func applyActionToValue(rule repository.TagEditRule, act repository.TagEditAction, value string) string {
	if act.Action != "replace" {
		return ""
	}
	return applyReplaceMode(act.ReplaceMode, rule.TextCondition, rule.Pattern, act.Replacement, value)
}

func nestedBlocksAllNonEmpty(obj domain.GraphicObject, rule repository.TagEditRule, meta SolutionMetadata) bool {
	start := time.Now()
	ok := true
	defer func() {
		slog.Debug("nestedBlocksAllNonEmpty done", "object_id", obj.Id, "ok", ok, "duration_ms", time.Since(start).Milliseconds())
	}()
	if !rule.CheckProperties && !rule.CheckCommands && !rule.CheckProtections {
		return true
	}
	if rule.CheckProperties {
		found := false
		for _, to := range obj.TechnologyObject {
			for _, prop := range to.TechnologyProperty {
				if !isTagEmpty(prop.Tag) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			ok = false
			return false
		}
	}
	if rule.CheckCommands {
		found := false
		for _, cmd := range meta.Commands {
			if !matchesTarget(cmd.Target, obj) {
				continue
			}
			if !isTagEmpty(cmd.Tag) {
				found = true
				break
			}
		}
		if !found {
			ok = false
			return false
		}
	}
	if rule.CheckProtections {
		found := false
		for _, auto := range meta.Automations {
			if !matchesTarget(auto.Target, obj) {
				continue
			}
			if !isTagEmpty(auto.Tag) {
				found = true
				break
			}
		}
		if !found {
			ok = false
			return false
		}
	}
	return true
}

func firstTechnologyObject(obj *domain.GraphicObject) *domain.TechnologyObject {
	if obj == nil || len(obj.TechnologyObject) == 0 {
		return nil
	}
	return &obj.TechnologyObject[0]
}

func resolveTagEditCheckValue(rule repository.TagEditRule, obj domain.GraphicObject, meta SolutionMetadata) string {
	cr := repository.ComplianceRule{
		ParameterScope: rule.ParameterScope,
		EntityTemplate: rule.EntityTemplate,
		Parameter:      rule.Parameter,
		ValueSource:    rule.ValueSource,
	}
	return resolveComplianceValues(cr, meta.asSolution(), &obj, firstTechnologyObject(&obj))
}

func ruleMatchesObject(rule repository.TagEditRule, obj domain.GraphicObject, meta SolutionMetadata) bool {
	if rule.Template == "" || obj.TechnologyTemplate() != rule.Template {
		return false
	}
	value := resolveTagEditCheckValue(rule, obj, meta)
	if !matchTagCondition(rule.TextCondition, rule.Pattern, value) {
		return false
	}
	return nestedBlocksAllNonEmpty(obj, rule, meta)
}

func setScalarField(field, value string, tag, name, description, id, externalCode *string) {
	switch field {
	case "Tag":
		*tag = value
	case "Name":
		*name = value
	case "Description":
		*description = value
	case "Id":
		if strings.TrimSpace(value) != "" {
			*id = value
		}
	case "ExternalCode":
		*externalCode = value
	}
}

func applyToTechnologyProperties(props []domain.TechnologyProperty, parameter, valueSource, next string) ([]domain.TechnologyProperty, bool) {
	changed := false
	for i := range props {
		if props[i].Name != parameter && props[i].Template != parameter {
			continue
		}
		if valueSource == "tag" {
			if props[i].Tag != next {
				props[i].Tag = next
				changed = true
			}
			continue
		}
		if props[i].StringValue != next || props[i].RealValue != 0 || props[i].IntegerValue != 0 || props[i].BooleanValue != "" {
			props[i].StringValue = next
			props[i].RealValue = 0
			props[i].IntegerValue = 0
			props[i].BooleanValue = ""
			changed = true
		}
	}
	return props, changed
}

func applyActionToObject(obj *domain.GraphicObject, rule repository.TagEditRule, act repository.TagEditAction) bool {
	to := firstTechnologyObject(obj)
	cr := repository.ComplianceRule{
		ParameterScope: act.ParameterScope,
		EntityTemplate: act.EntityTemplate,
		Parameter:      act.Parameter,
		ValueSource:    act.ValueSource,
	}
	var current string
	if act.ParameterScope == "property" {
		current = resolvePropertyComplianceField(cr, to)
	} else {
		current = resolveObjectComplianceField(cr, obj, to)
	}
	next := applyActionToValue(rule, act, current)
	if current == next {
		return false
	}
	if act.ParameterScope == "property" {
		if to == nil {
			return false
		}
		props, changed := applyToTechnologyProperties(to.TechnologyProperty, act.Parameter, act.ValueSource, next)
		to.TechnologyProperty = props
		return changed
	}
	if isObjectScalarParameter(act.Parameter) {
		if act.Parameter == "Tag" {
			if obj.Tag != next {
				obj.Tag = next
			}
			if to != nil {
				to.Tag = next
			}
			return true
		}
		if act.Parameter == "Name" {
			obj.Name = next
			if to != nil {
				to.Name = next
			}
			return true
		}
		if act.Parameter == "Description" {
			obj.Description = next
			if to != nil {
				to.Description = next
			}
			return true
		}
		if act.Parameter == "ExternalCode" {
			obj.ExternalCode = next
			if to != nil {
				to.ExternalCode = next
			}
			return true
		}
		if act.Parameter == "Id" && to != nil && strings.TrimSpace(next) != "" {
			to.Id = next
			return true
		}
		return false
	}
	if to == nil {
		return false
	}
	props, changed := applyToTechnologyProperties(to.TechnologyProperty, act.Parameter, act.ValueSource, next)
	to.TechnologyProperty = props
	return changed
}

func applyActionToCommands(meta *SolutionMetadata, obj domain.GraphicObject, rule repository.TagEditRule, act repository.TagEditAction) bool {
	changed := false
	for i := range meta.Commands {
		cmd := &meta.Commands[i]
		if act.EntityTemplate != "" && cmd.Template != act.EntityTemplate {
			continue
		}
		if !matchesTarget(cmd.Target, obj) {
			continue
		}
		cr := repository.ComplianceRule{
			Parameter:   act.Parameter,
			ValueSource: act.ValueSource,
		}
		current := resolveEntityComplianceField(cr, cmd.Tag, cmd.Name, cmd.Description, cmd.Id, cmd.ExternalCode, cmd.TechnologyProperty)
		next := applyActionToValue(rule, act, current)
		if current == next {
			continue
		}
		if isObjectScalarParameter(act.Parameter) {
			setScalarField(act.Parameter, next, &cmd.Tag, &cmd.Name, &cmd.Description, &cmd.Id, &cmd.ExternalCode)
			changed = true
			continue
		}
		props, propChanged := applyToTechnologyProperties(cmd.TechnologyProperty, act.Parameter, act.ValueSource, next)
		cmd.TechnologyProperty = props
		if propChanged {
			changed = true
		}
	}
	return changed
}

func applyActionToProtections(meta *SolutionMetadata, obj domain.GraphicObject, rule repository.TagEditRule, act repository.TagEditAction) bool {
	changed := false
	for i := range meta.Automations {
		auto := &meta.Automations[i]
		if act.EntityTemplate != "" && auto.Template != act.EntityTemplate {
			continue
		}
		if !matchesTarget(auto.Target, obj) {
			continue
		}
		cr := repository.ComplianceRule{
			Parameter:   act.Parameter,
			ValueSource: act.ValueSource,
		}
		current := resolveEntityComplianceField(cr, auto.Tag, auto.Name, auto.Description, auto.Id, auto.ExternalCode, auto.TechnologyProperty)
		next := applyActionToValue(rule, act, current)
		if current == next {
			continue
		}
		if isObjectScalarParameter(act.Parameter) {
			setScalarField(act.Parameter, next, &auto.Tag, &auto.Name, &auto.Description, &auto.Id, &auto.ExternalCode)
			changed = true
			continue
		}
		props, propChanged := applyToTechnologyProperties(auto.TechnologyProperty, act.Parameter, act.ValueSource, next)
		auto.TechnologyProperty = props
		if propChanged {
			changed = true
		}
	}
	return changed
}

func applyTagEditToObject(obj domain.GraphicObject, rules []repository.TagEditRule, meta *SolutionMetadata) (domain.GraphicObject, bool, bool) {
	start := time.Now()
	var objectChanged bool
	var metaChanged bool
	defer func() {
		slog.Debug("applyTagEditToObject done", "object_id", obj.Id, "changed", objectChanged, "meta_changed", metaChanged, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagEditToObject start", "object_id", obj.Id)
	if meta == nil {
		empty := SolutionMetadata{}
		meta = &empty
	}
	for i := range rules {
		rule := rules[i]
		ensureTagEditActions(&rule)
		if !ruleMatchesObject(rule, obj, *meta) {
			continue
		}
		for _, act := range rule.Actions {
			switch act.ParameterScope {
			case "command":
				if applyActionToCommands(meta, obj, rule, act) {
					metaChanged = true
				}
			case "protection":
				if applyActionToProtections(meta, obj, rule, act) {
					metaChanged = true
				}
			default:
				if applyActionToObject(&obj, rule, act) {
					objectChanged = true
				}
			}
		}
	}
	return obj, objectChanged, metaChanged
}

func applyTagRulesToValue(rules []repository.TagEditRule, tag string, obj domain.GraphicObject, meta SolutionMetadata) string {
	obj.Tag = tag
	if len(obj.TechnologyObject) > 0 {
		obj.TechnologyObject[0].Tag = tag
	}
	updated, _, _ := applyTagEditToObject(obj, rules, &meta)
	return updated.Tag
}

func (s *SolutionService) loadTagEditMetadata(ctx context.Context, solutionID uuid.UUID) (SolutionMetadata, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("loadTagEditMetadata done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("loadTagEditMetadata start", "solution_id", solutionID)
	meta, err := s.loadSolutionMetadata(ctx, solutionID)
	if err != nil {
		return SolutionMetadata{}, err
	}
	metaChanges, listErr := s.store.ListMetadataWorkingChanges(ctx, solutionID)
	if listErr != nil {
		err = listErr
		return SolutionMetadata{}, err
	}
	applyMetadataChanges(&meta, metaChanges)
	return meta, nil
}

func countTagEditEntities(refs []tagEditObjectRef) int {
	return len(refs)
}

type tagEditObjectRef struct {
	schemeXMLID string
	obj         domain.GraphicObject
}

func (s *SolutionService) collectGraphicObjects(sol domain.Solution) []tagEditObjectRef {
	var refs []tagEditObjectRef
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				refs = append(refs, tagEditObjectRef{
					schemeXMLID: scheme.Id,
					obj:         obj,
				})
			}
		}
	}
	return refs
}

func (s *SolutionService) PreviewTagEdit(ctx context.Context, solutionID uuid.UUID) (TagEditPreview, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("PreviewTagEdit done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	rules, err := s.store.ListTagEditRules(ctx, UserIDFromContext(ctx))
	if err != nil {
		return TagEditPreview{}, err
	}
	if len(rules) == 0 {
		err = ErrTagEditNoRules
		return TagEditPreview{}, err
	}

	sol, err := s.loadSolutionWithChanges(ctx, solutionID, nil)
	if err != nil {
		return TagEditPreview{}, err
	}
	meta, err := s.loadTagEditMetadata(ctx, solutionID)
	if err != nil {
		return TagEditPreview{}, err
	}

	refs := s.collectGraphicObjects(sol)
	preview := TagEditPreview{TotalObjects: countTagEditEntities(refs)}
	for _, ref := range refs {
		_, objectChanged, metaChanged := applyTagEditToObject(ref.obj, rules, &meta)
		if objectChanged || metaChanged {
			preview.ChangedObjects++
		}
	}
	return preview, nil
}

func (s *SolutionService) StartTagEditJob(ctx context.Context, solutionID uuid.UUID) (uuid.UUID, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("StartTagEditJob done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Info("StartTagEditJob start", "solution_id", solutionID)

	rules, err := s.store.ListTagEditRules(ctx, UserIDFromContext(ctx))
	if err != nil {
		return uuid.Nil, err
	}
	if len(rules) == 0 {
		err = ErrTagEditNoRules
		return uuid.Nil, err
	}

	if s.isBackgroundScanEnabled(ctx, solutionID) {
		err = ErrBackgroundScanActive
		return uuid.Nil, err
	}

	key := solutionID.String()
	if _, loaded := duplicateScanJobLocks.Load(key); loaded {
		err = ErrDuplicateScanActive
		return uuid.Nil, err
	}
	activeDup, aerr := s.store.HasActiveComplianceDuplicateJob(ctx, solutionID)
	if aerr != nil {
		err = aerr
		return uuid.Nil, err
	}
	if activeDup {
		err = ErrDuplicateScanActive
		return uuid.Nil, err
	}

	activeTag, terr := s.store.HasActiveTagEditJob(ctx, solutionID)
	if terr != nil {
		err = terr
		return uuid.Nil, err
	}
	if activeTag {
		err = ErrTagEditJobActive
		return uuid.Nil, err
	}

	if _, loaded := tagEditJobLocks.LoadOrStore(key, true); loaded {
		err = ErrTagEditJobActive
		return uuid.Nil, err
	}

	objCount, _ := s.store.CountSolutionObjects(ctx, solutionID)
	total := objCount

	jobID := uuid.New()
	job := repository.TagEditJob{
		ID:           jobID,
		SolutionID:   solutionID,
		Status:       "pending",
		TotalObjects: total,
	}
	if err = s.store.InsertTagEditJob(ctx, job); err != nil {
		tagEditJobLocks.Delete(key)
		return uuid.Nil, err
	}

	go s.runTagEditJob(jobID, solutionID, rules)

	return jobID, nil
}

func (s *SolutionService) runTagEditJob(jobID, solutionID uuid.UUID, rules []repository.TagEditRule) {
	start := time.Now()
	defer func() {
		tagEditJobLocks.Delete(solutionID.String())
		slog.Info("runTagEditJob done", "job_id", jobID, "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Info("runTagEditJob start", "job_id", jobID, "solution_id", solutionID)

	ctx := context.Background()
	_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "running", 0, 0, "", nil)

	sol, err := s.loadSolutionWithChanges(ctx, solutionID, nil)
	if err != nil {
		finished := time.Now()
		_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", 0, 0, err.Error(), &finished)
		return
	}
	meta, err := s.loadTagEditMetadata(ctx, solutionID)
	if err != nil {
		finished := time.Now()
		_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", 0, 0, err.Error(), &finished)
		return
	}

	total := countTagEditEntities(s.collectGraphicObjects(sol))
	slog.Info("runTagEditJob objects", "job_id", jobID, "total", total)
	processed := 0
	changed := 0
	affectedSchemes := map[string]map[string]domain.GraphicObject{}
	var changedEntityIDs []string
	changedCommands := map[string]struct{}{}
	changedAutomations := map[string]struct{}{}
	originalCommandJSON := tagEditEntitiesJSON(meta.Commands)
	originalAutoJSON := tagEditEntitiesJSON(meta.Automations)

	for pi := range sol.Projects.Project {
		for si := range sol.Projects.Project[pi].GraphicScheme {
			scheme := &sol.Projects.Project[pi].GraphicScheme[si]
			for oi := range scheme.GraphicObject {
				updated, objectChanged, metaChanged := applyTagEditToObject(scheme.GraphicObject[oi], rules, &meta)
				processed++
				if objectChanged || metaChanged {
					changed++
				}
				if objectChanged {
					scheme.GraphicObject[oi] = updated
					changedEntityIDs = append(changedEntityIDs, updated.Id)
					if affectedSchemes[scheme.Id] == nil {
						affectedSchemes[scheme.Id] = map[string]domain.GraphicObject{}
					}
					affectedSchemes[scheme.Id][updated.Id] = updated
				}
				if processed%50 == 0 {
					_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "running", processed, changed, "", nil)
				}
			}
		}
	}

	for i := range meta.Commands {
		if commandJSONChanged(originalCommandJSON, i, meta.Commands[i]) {
			changedCommands[meta.Commands[i].Id] = struct{}{}
		}
	}
	for i := range meta.Automations {
		if commandJSONChanged(originalAutoJSON, i, meta.Automations[i]) {
			changedAutomations[meta.Automations[i].Id] = struct{}{}
		}
	}

	for schemeXMLID, objects := range affectedSchemes {
		sch, getErr := s.store.GetScheme(ctx, solutionID, schemeXMLID)
		if getErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, getErr.Error(), &finished)
			return
		}
		var scheme domain.GraphicScheme
		if unmarshalErr := json.Unmarshal(sch.Payload, &scheme); unmarshalErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, unmarshalErr.Error(), &finished)
			return
		}
		for i := range scheme.GraphicObject {
			if updated, ok := objects[scheme.GraphicObject[i].Id]; ok {
				scheme.GraphicObject[i] = updated
			}
		}
		updatedPayload, marshalErr := json.Marshal(scheme)
		if marshalErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, marshalErr.Error(), &finished)
			return
		}
		if sizeErr := checkSchemePayloadSize(schemeXMLID, updatedPayload); sizeErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, sizeErr.Error(), &finished)
			return
		}
		if updateErr := s.store.UpdateSchemePayload(ctx, solutionID, schemeXMLID, updatedPayload); updateErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, updateErr.Error(), &finished)
			return
		}
	}

	sol.ControlCommands.ControlCommand = meta.Commands
	sol.Automations.Automation = meta.Automations

	if changed > 0 {
		if persistErr := s.persistSolutionMetadata(ctx, solutionID, meta); persistErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, persistErr.Error(), &finished)
			return
		}
		solData, marshalErr := marshalLeanSolution(sol)
		if marshalErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, marshalErr.Error(), &finished)
			return
		}
		if updateErr := s.store.UpdateSolutionData(ctx, solutionID, solData, true); updateErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, updateErr.Error(), &finished)
			return
		}
		for schemeXMLID, objs := range affectedSchemes {
			for objID := range objs {
				if upsertErr := s.store.UpsertSolutionChangedObject(ctx, solutionID, "graphic_object", objID, schemeXMLID); upsertErr != nil {
					finished := time.Now()
					_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, upsertErr.Error(), &finished)
					return
				}
				if delAckErr := s.store.DeleteExportChangeAck(ctx, solutionID, "graphic_object", objID); delAckErr != nil {
					finished := time.Now()
					_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, delAckErr.Error(), &finished)
					return
				}
			}
		}
		for id := range changedCommands {
			if upsertErr := s.store.UpsertSolutionChangedObject(ctx, solutionID, "control_command", id, ""); upsertErr != nil {
				finished := time.Now()
				_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, upsertErr.Error(), &finished)
				return
			}
			if delAckErr := s.store.DeleteExportChangeAck(ctx, solutionID, "control_command", id); delAckErr != nil {
				finished := time.Now()
				_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, delAckErr.Error(), &finished)
				return
			}
		}
		for id := range changedAutomations {
			if upsertErr := s.store.UpsertSolutionChangedObject(ctx, solutionID, "automation", id, ""); upsertErr != nil {
				finished := time.Now()
				_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, upsertErr.Error(), &finished)
				return
			}
			if delAckErr := s.store.DeleteExportChangeAck(ctx, solutionID, "automation", id); delAckErr != nil {
				finished := time.Now()
				_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, delAckErr.Error(), &finished)
				return
			}
		}
		if mergeErr := s.store.MergeWorkingIntoChangedObjects(ctx, solutionID); mergeErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, mergeErr.Error(), &finished)
			return
		}
		if clearErr := s.store.ClearWorkingChanges(ctx, solutionID); clearErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, clearErr.Error(), &finished)
			return
		}
	}

	if len(changedEntityIDs) > 0 {
		if draftErr := s.store.DeleteDraftChangesForObjectIDs(ctx, solutionID, changedEntityIDs); draftErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, draftErr.Error(), &finished)
			return
		}
	}

	if tagEditMarksSolutionDirty(changed) {
		if dirtyErr := s.store.SetSolutionDirty(ctx, solutionID, true); dirtyErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, dirtyErr.Error(), &finished)
			return
		}
	}

	finished := time.Now()
	_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "completed", processed, changed, "", &finished)
	s.BumpSolutionGen(ctx, solutionID)
	affectedSchemes = nil
	changedEntityIDs = nil
	sol = domain.Solution{}
	if changed > 0 {
		s.triggerBackgroundFullRebuild(ctx, solutionID)
	}
}

func (s *SolutionService) GetTagEditJob(ctx context.Context, solutionID, jobID uuid.UUID) (repository.TagEditJob, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetTagEditJob done", "job_id", jobID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	job, err := s.store.GetTagEditJob(ctx, jobID)
	if err != nil {
		return repository.TagEditJob{}, err
	}
	if job.SolutionID != solutionID {
		err = fmt.Errorf("job not found")
		return repository.TagEditJob{}, err
	}
	return job, nil
}
