package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

const CatalogVersion = 3

var baseComplianceParameters = []string{"Description", "ExternalCode", "Id", "Name", "Tag"}

func withBaseParameters(stored []string) []string {
	seen := map[string]bool{}
	for _, name := range baseComplianceParameters {
		seen[name] = true
	}
	result := make([]string, 0, len(baseComplianceParameters)+len(stored))
	result = append(result, baseComplianceParameters...)
	for _, name := range stored {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func (s *SolutionService) buildAndPersistSolutionIndexes(ctx context.Context, solutionID uuid.UUID, sol domain.Solution) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("buildAndPersistSolutionIndexes done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("buildAndPersistSolutionIndexes start", "solution_id", solutionID)

	err = s.store.WithSolutionIndexLock(ctx, solutionID, func(ctx context.Context, q repository.Querier) error {
		if err := s.store.DeleteSolutionIndexesQ(ctx, q, solutionID); err != nil {
			return err
		}

		typeSchemas := buildTypeSchemas(solutionID, sol)
		objects := buildSolutionObjects(solutionID, sol)
		targets := buildCommandTargets(solutionID, sol)
		paramRows := buildTemplateParameters(solutionID, sol)
		entityRows := buildTemplateEntities(solutionID, sol)

		if err := s.store.BulkUpsertTypeSchemas(ctx, q, typeSchemas); err != nil {
			return err
		}
		if err := s.store.BulkUpsertSolutionObjects(ctx, q, objects); err != nil {
			return err
		}
		if err := s.store.BulkUpsertCommandTargets(ctx, q, targets); err != nil {
			return err
		}
		if err := s.store.BulkUpsertTemplateParameters(ctx, q, paramRows); err != nil {
			return err
		}
		if err := s.store.BulkUpsertTemplateEntities(ctx, q, entityRows); err != nil {
			return err
		}
		return s.store.SetSolutionIndexVersion(ctx, q, solutionID, CatalogVersion)
	})
	return err
}

func buildTypeSchemas(solutionID uuid.UUID, sol domain.Solution) []repository.SolutionTypeSchemaRow {
	seen := map[string]repository.SolutionTypeSchemaRow{}

	for _, tp := range sol.TechnologyPassports.TechnologyPassport {
		if tp.Template == "" {
			continue
		}
		payload, _ := json.Marshal(tp)
		seen[tp.Template] = repository.SolutionTypeSchemaRow{
			SolutionID: solutionID,
			Template:   tp.Template,
			Schema:     payload,
			Source:     "passport",
		}
	}

	for _, tso := range sol.TechnologyStandaloneObjects.TechnologyStandaloneObject {
		if tso.Template == "" {
			continue
		}
		if _, ok := seen[tso.Template]; ok {
			continue
		}
		payload, _ := json.Marshal(tso)
		seen[tso.Template] = repository.SolutionTypeSchemaRow{
			SolutionID: solutionID,
			Template:   tso.Template,
			Schema:     payload,
			Source:     "standalone",
		}
	}

	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				tmpl := obj.TechnologyTemplate()
				if tmpl == "" || len(obj.TechnologyObject) == 0 {
					continue
				}
				if _, ok := seen[tmpl]; ok {
					continue
				}
				payload, _ := json.Marshal(obj.TechnologyObject[0])
				seen[tmpl] = repository.SolutionTypeSchemaRow{
					SolutionID: solutionID,
					Template:   tmpl,
					Schema:     payload,
					Source:     "inferred",
				}
			}
		}
	}

	result := make([]repository.SolutionTypeSchemaRow, 0, len(seen))
	for _, row := range seen {
		result = append(result, row)
	}
	return result
}

func buildSolutionObjects(solutionID uuid.UUID, sol domain.Solution) []repository.SolutionObjectRow {
	start := time.Now()
	seen := map[string]bool{}
	var result []repository.SolutionObjectRow

	slog.Debug("buildSolutionObjects start", "solution_id", solutionID)
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				if obj.Id == "" || seen[obj.Id] {
					continue
				}
				seen[obj.Id] = true
				var to *domain.TechnologyObject
				techID := ""
				if len(obj.TechnologyObject) > 0 {
					to = &obj.TechnologyObject[0]
					techID = to.Id
				}
				payload, _ := json.Marshal(obj.TechnologyObject)
				result = append(result, repository.SolutionObjectRow{
					SolutionID:         solutionID,
					GraphicObjectID:    obj.Id,
					TechnologyObjectID: techID,
					SchemeXMLID:        scheme.Id,
					Template:           objectDisplayType(obj, to),
					Tag:                objectDisplayTag(obj, to),
					Name:               objectDisplayName(obj, to),
					TargetKeys:         objectTargetKeys(obj),
					TechnologyPayload:  payload,
				})
			}
		}
	}
	slog.Debug("buildSolutionObjects done",
		"solution_id", solutionID,
		"count", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return result
}

func buildCommandTargets(solutionID uuid.UUID, sol domain.Solution) []repository.SolutionCommandTargetRow {
	var result []repository.SolutionCommandTargetRow
	for _, cmd := range sol.ControlCommands.ControlCommand {
		if cmd.Target == "" {
			continue
		}
		result = append(result, repository.SolutionCommandTargetRow{
			SolutionID:  solutionID,
			TargetKey:   cmd.Target,
			EntityType:  "control_command",
			EntityXMLID: cmd.Id,
		})
	}
	for _, auto := range sol.Automations.Automation {
		if auto.Target == "" {
			continue
		}
		result = append(result, repository.SolutionCommandTargetRow{
			SolutionID:  solutionID,
			TargetKey:   auto.Target,
			EntityType:  "automation",
			EntityXMLID: auto.Id,
		})
	}
	return result
}

func propertyKey(p domain.TechnologyProperty) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Template
}

func buildTemplateParameters(solutionID uuid.UUID, sol domain.Solution) []repository.SolutionTemplateParametersRow {
	start := time.Now()
	defer func() {
		slog.Debug("buildTemplateParameters done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("buildTemplateParameters start", "solution_id", solutionID)

	objectProps := map[string]map[string]bool{}
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				tmpl := obj.TechnologyTemplate()
				if tmpl == "" || len(obj.TechnologyObject) == 0 {
					continue
				}
				seen := objectProps[tmpl]
				if seen == nil {
					seen = map[string]bool{}
					objectProps[tmpl] = seen
				}
				for _, p := range obj.TechnologyObject[0].TechnologyProperty {
					key := propertyKey(p)
					if key == "" {
						continue
					}
					seen[key] = true
				}
			}
		}
	}

	commandProps := map[string]map[string]bool{}
	for _, cmd := range sol.ControlCommands.ControlCommand {
		if cmd.Template == "" {
			continue
		}
		seen := commandProps[cmd.Template]
		if seen == nil {
			seen = map[string]bool{}
			commandProps[cmd.Template] = seen
		}
		for _, p := range cmd.TechnologyProperty {
			key := propertyKey(p)
			if key == "" {
				continue
			}
			seen[key] = true
		}
	}

	protectionProps := map[string]map[string]bool{}
	for _, auto := range sol.Automations.Automation {
		if auto.Template == "" {
			continue
		}
		seen := protectionProps[auto.Template]
		if seen == nil {
			seen = map[string]bool{}
			protectionProps[auto.Template] = seen
		}
		for _, p := range auto.TechnologyProperty {
			key := propertyKey(p)
			if key == "" {
				continue
			}
			seen[key] = true
		}
	}

	var rows []repository.SolutionTemplateParametersRow
	appendSorted := func(scope, owner string, keys map[string]bool, withBase bool) {
		extra := make([]string, 0, len(keys))
		for k := range keys {
			extra = append(extra, k)
		}
		sort.Strings(extra)
		params := extra
		if withBase {
			params = withBaseParameters(extra)
		}
		rows = append(rows, repository.SolutionTemplateParametersRow{
			SolutionID:    solutionID,
			Scope:         scope,
			OwnerTemplate: owner,
			Parameters:    params,
		})
	}

	for tmpl, keys := range objectProps {
		appendSorted("property", tmpl, keys, false)
		appendSorted("object", tmpl, keys, true)
	}
	for tmpl, keys := range commandProps {
		appendSorted("command", tmpl, keys, true)
	}
	for tmpl, keys := range protectionProps {
		appendSorted("protection", tmpl, keys, true)
	}
	return rows
}

func buildTemplateEntities(solutionID uuid.UUID, sol domain.Solution) []repository.SolutionTemplateEntitiesRow {
	start := time.Now()
	defer func() {
		slog.Debug("buildTemplateEntities done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("buildTemplateEntities start", "solution_id", solutionID)

	targetToTemplates := map[string]map[string]struct{}{}
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				objectTemplate := obj.TechnologyTemplate()
				if objectTemplate == "" {
					continue
				}
				for _, key := range objectTargetKeys(obj) {
					set := targetToTemplates[key]
					if set == nil {
						set = map[string]struct{}{}
						targetToTemplates[key] = set
					}
					set[objectTemplate] = struct{}{}
				}
			}
		}
	}

	commandByObject := map[string]map[string]struct{}{}
	for _, cmd := range sol.ControlCommands.ControlCommand {
		if cmd.Template == "" || cmd.Target == "" {
			continue
		}
		for objectTemplate := range targetToTemplates[cmd.Target] {
			set := commandByObject[objectTemplate]
			if set == nil {
				set = map[string]struct{}{}
				commandByObject[objectTemplate] = set
			}
			set[cmd.Template] = struct{}{}
		}
	}

	protectionByObject := map[string]map[string]struct{}{}
	for _, auto := range sol.Automations.Automation {
		if auto.Template == "" || auto.Target == "" {
			continue
		}
		for objectTemplate := range targetToTemplates[auto.Target] {
			set := protectionByObject[objectTemplate]
			if set == nil {
				set = map[string]struct{}{}
				protectionByObject[objectTemplate] = set
			}
			set[auto.Template] = struct{}{}
		}
	}

	var rows []repository.SolutionTemplateEntitiesRow
	appendRows := func(scope string, byObject map[string]map[string]struct{}) {
		for objectTemplate, set := range byObject {
			list := make([]string, 0, len(set))
			for tmpl := range set {
				list = append(list, tmpl)
			}
			sort.Strings(list)
			rows = append(rows, repository.SolutionTemplateEntitiesRow{
				SolutionID:      solutionID,
				ObjectTemplate:  objectTemplate,
				Scope:           scope,
				EntityTemplates: list,
			})
		}
	}
	appendRows("command", commandByObject)
	appendRows("protection", protectionByObject)
	return rows
}

func lookupTemplateParameters(rows []repository.SolutionTemplateParametersRow, scope, ownerTemplate string) []string {
	for _, row := range rows {
		if row.Scope == scope && row.OwnerTemplate == ownerTemplate {
			if row.Parameters == nil {
				return []string{}
			}
			return append([]string(nil), row.Parameters...)
		}
	}
	return []string{}
}

func lookupTemplateEntities(rows []repository.SolutionTemplateEntitiesRow, objectTemplate, scope string) []string {
	for _, row := range rows {
		if row.ObjectTemplate == objectTemplate && row.Scope == scope {
			if row.EntityTemplates == nil {
				return []string{}
			}
			return append([]string(nil), row.EntityTemplates...)
		}
	}
	return []string{}
}

func (s *SolutionService) GetTypeSchema(ctx context.Context, solutionID uuid.UUID, template string) (json.RawMessage, string, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetTypeSchema done", "solution_id", solutionID, "template", template, "duration_ms", time.Since(start).Milliseconds())
	}()

	row, err := s.store.GetTypeSchema(ctx, solutionID, template)
	if err != nil {
		return nil, "", err
	}
	return row.Schema, row.Source, nil
}

func (s *SolutionService) GetObjectCommands(ctx context.Context, solutionID uuid.UUID, graphicObjectID string) (ObjectCommands, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetObjectCommands done", "solution_id", solutionID, "object_id", graphicObjectID, "duration_ms", time.Since(start).Milliseconds())
	}()

	details, err := s.GetObjectDetails(ctx, solutionID, graphicObjectID, nil)
	if err != nil {
		return ObjectCommands{}, err
	}
	return ObjectCommands{
		Automations: details.Automations,
		Commands:    details.Commands,
	}, nil
}

type ObjectCommands struct {
	Automations []domain.Automation     `json:"automations"`
	Commands    []domain.ControlCommand `json:"commands"`
}

func (s *SolutionService) RebuildSolutionIndexes(ctx context.Context, solutionID uuid.UUID) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("RebuildSolutionIndexes done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("RebuildSolutionIndexes start", "solution_id", solutionID)

	sol, loadErr := s.loadSolution(ctx, solutionID)
	if loadErr != nil {
		err = loadErr
		return err
	}
	err = s.buildAndPersistSolutionIndexes(ctx, solutionID, sol)
	if err == nil {
		s.BumpSolutionGen(ctx, solutionID)
	}
	return err
}

func (s *SolutionService) ensureSolutionIndexes(ctx context.Context, solutionID uuid.UUID) error {
	start := time.Now()
	var err error
	rebuilt := false
	defer func() {
		slog.Info("ensureSolutionIndexes done",
			"solution_id", solutionID,
			"rebuilt", rebuilt,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err)
	}()
	slog.Debug("ensureSolutionIndexes start", "solution_id", solutionID)

	version, verErr := s.store.GetSolutionIndexVersion(ctx, nil, solutionID)
	if verErr != nil {
		err = verErr
		return err
	}
	if version >= CatalogVersion {
		slog.Debug("ensureSolutionIndexes skip", "solution_id", solutionID, "index_version", version)
		return nil
	}

	err = s.store.WithSolutionIndexLock(ctx, solutionID, func(ctx context.Context, q repository.Querier) error {
		lockedVersion, lockedErr := s.store.GetSolutionIndexVersion(ctx, q, solutionID)
		if lockedErr != nil {
			return lockedErr
		}
		if lockedVersion >= CatalogVersion {
			slog.Debug("ensureSolutionIndexes skip locked", "solution_id", solutionID, "index_version", lockedVersion)
			return nil
		}
		sol, loadErr := s.loadSolution(ctx, solutionID)
		if loadErr != nil {
			return loadErr
		}
		if delErr := s.store.DeleteSolutionIndexesQ(ctx, q, solutionID); delErr != nil {
			return delErr
		}
		typeSchemas := buildTypeSchemas(solutionID, sol)
		objects := buildSolutionObjects(solutionID, sol)
		targets := buildCommandTargets(solutionID, sol)
		paramRows := buildTemplateParameters(solutionID, sol)
		entityRows := buildTemplateEntities(solutionID, sol)
		if upsertErr := s.store.BulkUpsertTypeSchemas(ctx, q, typeSchemas); upsertErr != nil {
			return upsertErr
		}
		if upsertErr := s.store.BulkUpsertSolutionObjects(ctx, q, objects); upsertErr != nil {
			return upsertErr
		}
		if upsertErr := s.store.BulkUpsertCommandTargets(ctx, q, targets); upsertErr != nil {
			return upsertErr
		}
		if upsertErr := s.store.BulkUpsertTemplateParameters(ctx, q, paramRows); upsertErr != nil {
			return upsertErr
		}
		if upsertErr := s.store.BulkUpsertTemplateEntities(ctx, q, entityRows); upsertErr != nil {
			return upsertErr
		}
		if setErr := s.store.SetSolutionIndexVersion(ctx, q, solutionID, CatalogVersion); setErr != nil {
			return setErr
		}
		rebuilt = true
		return nil
	})
	if err == nil && rebuilt {
		s.BumpSolutionGen(ctx, solutionID)
	}
	return err
}
