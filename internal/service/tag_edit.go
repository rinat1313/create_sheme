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
	ErrTagEditJobActive   = errors.New("tag edit job already running for this solution")
	ErrTagEditNoRules     = errors.New("no tag edit rules configured")
	ErrTagEditEmptyPattern = errors.New("pattern must not be empty")
)

var tagEditJobLocks sync.Map

func tagEditMarksSolutionDirty(changed int) bool {
	return changed > 0
}

type TagEditRuleInput struct {
	ID            string `json:"id"`
	TextCondition string `json:"text_condition"`
	Pattern       string `json:"pattern"`
	Action        string `json:"action"`
	Replacement   string `json:"replacement"`
	SortOrder     int    `json:"sort_order"`
}

type TagEditPreview struct {
	TotalObjects   int `json:"total_objects"`
	ChangedObjects int `json:"changed_objects"`
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

	rule := repository.TagEditRule{
		ID:            uuid.New(),
		UserID:        UserIDFromContext(ctx),
		TextCondition: normalizeTagTextCondition(input.TextCondition),
		Pattern:       strings.TrimSpace(input.Pattern),
		Action:        normalizeTagAction(input.Action),
		Replacement:   input.Replacement,
		SortOrder:     input.SortOrder,
	}
	if rule.Pattern == "" {
		err = ErrTagEditEmptyPattern
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
	rule := repository.TagEditRule{
		ID:            id,
		UserID:        UserIDFromContext(ctx),
		TextCondition: normalizeTagTextCondition(input.TextCondition),
		Pattern:       strings.TrimSpace(input.Pattern),
		Action:        normalizeTagAction(input.Action),
		Replacement:   input.Replacement,
		SortOrder:     input.SortOrder,
	}
	if rule.Pattern == "" {
		err = ErrTagEditEmptyPattern
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

func normalizeTagTextCondition(cond string) string {
	switch cond {
	case "starts_with", "ends_with":
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

func matchTagCondition(textCondition, pattern, tag string) bool {
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

func applyTagRulesToValue(rules []repository.TagEditRule, tag string) string {
	start := time.Now()
	defer func() {
		slog.Debug("applyTagRulesToValue done", "rules", len(rules), "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagRulesToValue start", "rules", len(rules))
	value := tag
	for _, rule := range rules {
		if !matchTagCondition(rule.TextCondition, rule.Pattern, value) {
			continue
		}
		switch rule.Action {
		case "replace":
			value = rule.Replacement
		default:
			value = ""
		}
	}
	return value
}

func applyTagEditToTechnologyProperties(props []domain.TechnologyProperty, rules []repository.TagEditRule) ([]domain.TechnologyProperty, bool) {
	start := time.Now()
	var changed bool
	defer func() {
		slog.Debug("applyTagEditToTechnologyProperties done", "props", len(props), "changed", changed, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagEditToTechnologyProperties start", "props", len(props))
	for i := range props {
		newTag := applyTagRulesToValue(rules, props[i].Tag)
		if newTag != props[i].Tag {
			props[i].Tag = newTag
			changed = true
		}
	}
	return props, changed
}

func applyTagEditToObject(obj domain.GraphicObject, rules []repository.TagEditRule) (domain.GraphicObject, bool) {
	start := time.Now()
	var changed bool
	defer func() {
		slog.Debug("applyTagEditToObject done", "object_id", obj.Id, "changed", changed, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagEditToObject start", "object_id", obj.Id)
	newTag := applyTagRulesToValue(rules, obj.Tag)
	if newTag != obj.Tag {
		obj.Tag = newTag
		changed = true
	}
	if len(obj.TechnologyObject) > 0 {
		newTOTag := applyTagRulesToValue(rules, obj.TechnologyObject[0].Tag)
		if newTOTag != obj.TechnologyObject[0].Tag {
			obj.TechnologyObject[0].Tag = newTOTag
			changed = true
		}
		props, propsChanged := applyTagEditToTechnologyProperties(obj.TechnologyObject[0].TechnologyProperty, rules)
		if propsChanged {
			obj.TechnologyObject[0].TechnologyProperty = props
			changed = true
		}
	}
	return obj, changed
}

func applyTagEditToControlCommand(cmd domain.ControlCommand, rules []repository.TagEditRule) (domain.ControlCommand, bool) {
	start := time.Now()
	var changed bool
	defer func() {
		slog.Debug("applyTagEditToControlCommand done", "command_id", cmd.Id, "changed", changed, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagEditToControlCommand start", "command_id", cmd.Id)
	newTag := applyTagRulesToValue(rules, cmd.Tag)
	if newTag != cmd.Tag {
		cmd.Tag = newTag
		changed = true
	}
	props, propsChanged := applyTagEditToTechnologyProperties(cmd.TechnologyProperty, rules)
	if propsChanged {
		cmd.TechnologyProperty = props
		changed = true
	}
	return cmd, changed
}

func applyTagEditToAutomation(auto domain.Automation, rules []repository.TagEditRule) (domain.Automation, bool) {
	start := time.Now()
	var changed bool
	defer func() {
		slog.Debug("applyTagEditToAutomation done", "automation_id", auto.Id, "changed", changed, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("applyTagEditToAutomation start", "automation_id", auto.Id)
	newTag := applyTagRulesToValue(rules, auto.Tag)
	if newTag != auto.Tag {
		auto.Tag = newTag
		changed = true
	}
	props, propsChanged := applyTagEditToTechnologyProperties(auto.TechnologyProperty, rules)
	if propsChanged {
		auto.TechnologyProperty = props
		changed = true
	}
	return auto, changed
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

func countTagEditEntities(refs []tagEditObjectRef, meta SolutionMetadata) int {
	return len(refs) + len(meta.Commands) + len(meta.Automations)
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
	preview := TagEditPreview{TotalObjects: countTagEditEntities(refs, meta)}
	for _, ref := range refs {
		_, changed := applyTagEditToObject(ref.obj, rules)
		if changed {
			preview.ChangedObjects++
		}
	}
	for _, cmd := range meta.Commands {
		_, changed := applyTagEditToControlCommand(cmd, rules)
		if changed {
			preview.ChangedObjects++
		}
	}
	for _, auto := range meta.Automations {
		_, changed := applyTagEditToAutomation(auto, rules)
		if changed {
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

	if _, loaded := tagEditJobLocks.LoadOrStore(key, true); loaded {
		err = ErrTagEditJobActive
		return uuid.Nil, err
	}

	meta, err := s.loadTagEditMetadata(ctx, solutionID)
	if err != nil {
		tagEditJobLocks.Delete(key)
		return uuid.Nil, err
	}
	objCount, _ := s.store.CountSolutionObjects(ctx, solutionID)
	total := objCount + len(meta.Commands) + len(meta.Automations)

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

	total := countTagEditEntities(s.collectGraphicObjects(sol), meta)
	processed := 0
	changed := 0
	affectedSchemes := map[string]map[string]domain.GraphicObject{}
	var changedEntityIDs []string
	metadataChanged := false

	for pi := range sol.Projects.Project {
		for si := range sol.Projects.Project[pi].GraphicScheme {
			scheme := &sol.Projects.Project[pi].GraphicScheme[si]
			for oi := range scheme.GraphicObject {
				updated, objectChanged := applyTagEditToObject(scheme.GraphicObject[oi], rules)
				processed++
				if objectChanged {
					changed++
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

	for i, cmd := range meta.Commands {
		updated, cmdChanged := applyTagEditToControlCommand(cmd, rules)
		processed++
		if cmdChanged {
			changed++
			metadataChanged = true
			meta.Commands[i] = updated
			changedEntityIDs = append(changedEntityIDs, updated.Id)
		}
		if processed%50 == 0 || processed == total {
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "running", processed, changed, "", nil)
		}
	}

	for i, auto := range meta.Automations {
		updated, autoChanged := applyTagEditToAutomation(auto, rules)
		processed++
		if autoChanged {
			changed++
			metadataChanged = true
			meta.Automations[i] = updated
			changedEntityIDs = append(changedEntityIDs, updated.Id)
		}
		if processed%50 == 0 || processed == total {
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "running", processed, changed, "", nil)
		}
	}

	if metadataChanged {
		sol.ControlCommands.ControlCommand = meta.Commands
		sol.Automations.Automation = meta.Automations
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

	if metadataChanged {
		if persistErr := s.persistSolutionMetadata(ctx, solutionID, meta); persistErr != nil {
			finished := time.Now()
			_ = s.store.UpdateTagEditJobProgress(ctx, jobID, "failed", processed, changed, persistErr.Error(), &finished)
			return
		}
	}

	if changed > 0 {
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
