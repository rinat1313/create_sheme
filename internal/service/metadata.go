package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SolutionMetadata struct {
	Automations []domain.Automation     `json:"automations"`
	Commands    []domain.ControlCommand `json:"commands"`
}

func (m SolutionMetadata) asSolution() domain.Solution {
	return domain.Solution{
		Automations:     domain.Automations{Automation: m.Automations},
		ControlCommands: domain.ControlCommands{ControlCommand: m.Commands},
	}
}

func (s *SolutionService) loadSolutionMetadata(ctx context.Context, solutionID uuid.UUID) (SolutionMetadata, error) {
	start := time.Now()
	defer func() {
		slog.Info("loadSolutionMetadata done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()

	row, err := s.store.GetSolutionMetadata(ctx, solutionID)
	if err == nil {
		var meta SolutionMetadata
		if len(row.Automations) > 0 {
			_ = json.Unmarshal(row.Automations, &meta.Automations)
		}
		if len(row.ControlCommands) > 0 {
			_ = json.Unmarshal(row.ControlCommands, &meta.Commands)
		}
		return meta, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SolutionMetadata{}, err
	}

	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return SolutionMetadata{}, err
	}
	meta := SolutionMetadata{
		Automations: sol.Automations.Automation,
		Commands:    sol.ControlCommands.ControlCommand,
	}
	if err := s.persistSolutionMetadata(ctx, solutionID, meta); err != nil {
		return SolutionMetadata{}, err
	}
	return meta, nil
}

func (s *SolutionService) persistSolutionMetadata(ctx context.Context, solutionID uuid.UUID, meta SolutionMetadata) error {
	autoJSON, err := json.Marshal(meta.Automations)
	if err != nil {
		return err
	}
	cmdJSON, err := json.Marshal(meta.Commands)
	if err != nil {
		return err
	}
	return s.store.UpsertSolutionMetadata(ctx, solutionID, autoJSON, cmdJSON)
}

func applyMetadataChanges(meta *SolutionMetadata, changes []repository.ChangeRow) {
	for _, ch := range changes {
		switch ch.EntityType {
		case "control_command":
			applyMetadataCommandPatch(meta, ch)
		case "automation":
			applyMetadataAutomationPatch(meta, ch)
		}
	}
}

func applyMetadataCommandPatch(meta *SolutionMetadata, ch repository.ChangeRow) {
	var cmd domain.ControlCommand
	if err := json.Unmarshal(ch.Patch, &cmd); err != nil {
		return
	}
	for i := range meta.Commands {
		if meta.Commands[i].Id == ch.EntityXMLID {
			meta.Commands[i] = cmd
			return
		}
	}
}

func applyMetadataAutomationPatch(meta *SolutionMetadata, ch repository.ChangeRow) {
	var auto domain.Automation
	if err := json.Unmarshal(ch.Patch, &auto); err != nil {
		return
	}
	for i := range meta.Automations {
		if meta.Automations[i].Id == ch.EntityXMLID {
			meta.Automations[i] = auto
			return
		}
	}
}

func buildObjectDetailsFromMetadata(meta SolutionMetadata, obj domain.GraphicObject) ObjectDetails {
	return buildObjectDetailsFromMetadataAndTargets(meta, obj, nil)
}

func buildObjectDetailsFromMetadataAndTargets(meta SolutionMetadata, obj domain.GraphicObject, refs []repository.CommandTargetRef) ObjectDetails {
	details := ObjectDetails{
		GraphicObject:    obj,
		TechnologyObject: obj.TechnologyObject,
	}
	if len(refs) == 0 {
		for _, auto := range meta.Automations {
			if matchesTarget(auto.Target, obj) {
				details.Automations = append(details.Automations, auto)
			}
		}
		for _, cmd := range meta.Commands {
			if matchesTarget(cmd.Target, obj) {
				details.Commands = append(details.Commands, cmd)
			}
		}
		return details
	}

	cmdIDs := map[string]struct{}{}
	autoIDs := map[string]struct{}{}
	for _, ref := range refs {
		switch ref.EntityType {
		case "control_command":
			cmdIDs[ref.EntityXMLID] = struct{}{}
		case "automation":
			autoIDs[ref.EntityXMLID] = struct{}{}
		}
	}
	for _, auto := range meta.Automations {
		if _, ok := autoIDs[auto.Id]; ok {
			details.Automations = append(details.Automations, auto)
		}
	}
	for _, cmd := range meta.Commands {
		if _, ok := cmdIDs[cmd.Id]; ok {
			details.Commands = append(details.Commands, cmd)
		}
	}
	return details
}
