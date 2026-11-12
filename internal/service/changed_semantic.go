package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

func graphicObjectSemanticForm(obj domain.GraphicObject) domain.GraphicObject {
	out := obj
	out.Angle = ""
	out.IsVisibled = ""
	out.IsHorizontalReflected = ""
	out.IsVerticalReflected = ""
	out.IsReversed = ""
	out.AnchorPoint = nil
	out.Points = nil
	out.GraphicLabels = nil
	return out
}

func graphicObjectSemanticChange(baseline, patched domain.GraphicObject) bool {
	b, err := json.Marshal(graphicObjectSemanticForm(baseline))
	if err != nil {
		return true
	}
	p, err := json.Marshal(graphicObjectSemanticForm(patched))
	if err != nil {
		return true
	}
	return string(b) != string(p)
}

func (s *SolutionService) loadPersistedGraphicObject(ctx context.Context, solutionID uuid.UUID, schemeXMLID, objectID string) (domain.GraphicObject, bool, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("loadPersistedGraphicObject done",
			"solution_id", solutionID,
			"scheme_xml_id", schemeXMLID,
			"object_id", objectID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("loadPersistedGraphicObject start",
		"solution_id", solutionID,
		"scheme_xml_id", schemeXMLID,
		"object_id", objectID,
	)

	sch, err := s.store.GetScheme(ctx, solutionID, schemeXMLID)
	if err != nil {
		return domain.GraphicObject{}, false, err
	}
	var scheme domain.GraphicScheme
	if err = json.Unmarshal(sch.Payload, &scheme); err != nil {
		return domain.GraphicObject{}, false, err
	}
	for _, obj := range scheme.GraphicObject {
		if obj.Id == objectID {
			return obj, true, nil
		}
	}
	return domain.GraphicObject{}, false, nil
}

func (s *SolutionService) graphicObjectPatchSemantic(ctx context.Context, solutionID uuid.UUID, schemeXMLID, objectID string, patch json.RawMessage) (bool, error) {
	start := time.Now()
	var err error
	var semantic bool
	defer func() {
		slog.Debug("graphicObjectPatchSemantic done",
			"solution_id", solutionID,
			"object_id", objectID,
			"semantic", semantic,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("graphicObjectPatchSemantic start", "solution_id", solutionID, "object_id", objectID)

	var patched domain.GraphicObject
	if err = json.Unmarshal(patch, &patched); err != nil {
		return false, err
	}
	baseline, ok, err := s.loadPersistedGraphicObject(ctx, solutionID, schemeXMLID, objectID)
	if err != nil {
		return false, err
	}
	if !ok {
		semantic = true
		return semantic, nil
	}
	semantic = graphicObjectSemanticChange(baseline, patched)
	return semantic, nil
}

func (s *SolutionService) applyGraphicObjectChangedMarking(ctx context.Context, solutionID uuid.UUID, schemeXMLID, objectID string, patch json.RawMessage) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("applyGraphicObjectChangedMarking done",
			"solution_id", solutionID,
			"object_id", objectID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("applyGraphicObjectChangedMarking start", "solution_id", solutionID, "object_id", objectID)

	semantic, err := s.graphicObjectPatchSemantic(ctx, solutionID, schemeXMLID, objectID, patch)
	if err != nil {
		return err
	}
	if semantic {
		if err = s.store.UpsertSolutionChangedObject(ctx, solutionID, "graphic_object", objectID, schemeXMLID); err != nil {
			return err
		}
		if err = s.store.DeleteExportChangeAck(ctx, solutionID, "graphic_object", objectID); err != nil {
			return err
		}
		return nil
	}
	return s.store.DeleteSolutionChangedObjects(ctx, solutionID, "graphic_object", []string{objectID})
}

func (s *SolutionService) upsertDraftChange(ctx context.Context, id, sessionID uuid.UUID, schemeXMLID, entityType, entityXMLID string, patch json.RawMessage) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("upsertDraftChange done",
			"session_id", sessionID,
			"entity_type", entityType,
			"entity_xml_id", entityXMLID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("upsertDraftChange start", "session_id", sessionID, "entity_type", entityType, "entity_xml_id", entityXMLID)

	if err = s.store.UpsertDraftChange(ctx, id, sessionID, schemeXMLID, entityType, entityXMLID, patch); err != nil {
		return err
	}
	if entityType != "graphic_object" {
		return nil
	}
	solutionID, _, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	return s.applyGraphicObjectChangedMarking(ctx, solutionID, schemeXMLID, entityXMLID, patch)
}

func (s *SolutionService) flushDraftToWorking(ctx context.Context, solutionID, sessionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("flushDraftToWorking done",
			"solution_id", solutionID,
			"session_id", sessionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("flushDraftToWorking start", "solution_id", solutionID, "session_id", sessionID)

	var drafts []repository.ChangeRow
	drafts, err = s.store.ListDraftChanges(ctx, sessionID)
	if err != nil {
		return err
	}
	if err = s.store.FlushDraftToWorking(ctx, solutionID, sessionID); err != nil {
		return err
	}
	for _, ch := range drafts {
		if ch.EntityType != "graphic_object" {
			continue
		}
		if err = s.applyGraphicObjectChangedMarking(ctx, solutionID, ch.SchemeXMLID, ch.EntityXMLID, ch.Patch); err != nil {
			return err
		}
	}
	return nil
}

func (s *SolutionService) mergeSemanticWorkingIntoChangedObjects(ctx context.Context, solutionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("mergeSemanticWorkingIntoChangedObjects done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("mergeSemanticWorkingIntoChangedObjects start", "solution_id", solutionID)

	var changes []repository.ChangeRow
	changes, err = s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return err
	}
	for _, ch := range changes {
		if ch.EntityType != "graphic_object" {
			continue
		}
		var semantic bool
		semantic, err = s.graphicObjectPatchSemantic(ctx, solutionID, ch.SchemeXMLID, ch.EntityXMLID, ch.Patch)
		if err != nil {
			return err
		}
		if !semantic {
			continue
		}
		if err = s.store.UpsertSolutionChangedObject(ctx, solutionID, "graphic_object", ch.EntityXMLID, ch.SchemeXMLID); err != nil {
			return err
		}
	}
	return nil
}

func workingOrDraftGraphicObjectSemantic(ctx context.Context, svc *SolutionService, solutionID uuid.UUID, ch repository.ChangeRow) (bool, error) {
	if ch.EntityType != "graphic_object" {
		return false, nil
	}
	return svc.graphicObjectPatchSemantic(ctx, solutionID, ch.SchemeXMLID, ch.EntityXMLID, ch.Patch)
}
