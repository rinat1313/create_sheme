package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"create_sheme/internal/domain"
	"create_sheme/internal/repository"
)

const (
	schemePayloadSoftLimit = 240 << 20
	schemePayloadHardLimit = 900 << 20
)

func marshalLeanSolution(sol domain.Solution) (json.RawMessage, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("marshalLeanSolution done",
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("marshalLeanSolution start")
	sol = cloneSolutionSchemeStubs(sol)
	sol.StripSchemeGraphicObjects()
	var raw []byte
	raw, err = json.Marshal(sol)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func cloneSolutionSchemeStubs(sol domain.Solution) domain.Solution {
	if len(sol.Projects.Project) == 0 {
		return sol
	}
	projects := make([]domain.Project, len(sol.Projects.Project))
	copy(projects, sol.Projects.Project)
	for i := range projects {
		if len(projects[i].GraphicScheme) == 0 {
			continue
		}
		schemes := make([]domain.GraphicScheme, len(projects[i].GraphicScheme))
		copy(schemes, projects[i].GraphicScheme)
		projects[i].GraphicScheme = schemes
	}
	sol.Projects.Project = projects
	return sol
}

func checkSchemePayloadSize(schemeXMLID string, payload []byte) error {
	return checkSchemePayloadByteCount(schemeXMLID, len(payload))
}

func checkSchemePayloadByteCount(schemeXMLID string, n int) error {
	if n > schemePayloadHardLimit {
		return fmt.Errorf("scheme %s uncompressed payload %d bytes exceeds limit %d (~900MB); split the scheme or reduce GraphicObject data", schemeXMLID, n, schemePayloadHardLimit)
	}
	if n > schemePayloadSoftLimit {
		slog.Warn("scheme uncompressed payload near soft limit",
			"scheme_xml_id", schemeXMLID,
			"bytes", n,
			"soft_limit", schemePayloadSoftLimit,
			"hard_limit", schemePayloadHardLimit,
		)
	}
	return nil
}

func hydrateSolutionSchemes(sol *domain.Solution, schemes []repository.SchemeRow) (err error) {
	start := time.Now()
	defer func() {
		slog.Debug("hydrateSolutionSchemes done",
			"schemes", len(schemes),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("hydrateSolutionSchemes start", "schemes", len(schemes))
	if sol == nil || len(schemes) == 0 {
		return nil
	}
	type schemeKey struct {
		projectXMLID string
		xmlID        string
	}
	byKey := make(map[schemeKey]domain.GraphicScheme, len(schemes))
	for _, row := range schemes {
		var full domain.GraphicScheme
		if unmarshalErr := json.Unmarshal(row.Payload, &full); unmarshalErr != nil {
			err = fmt.Errorf("unmarshal scheme %s: %w", row.XMLID, unmarshalErr)
			return err
		}
		byKey[schemeKey{projectXMLID: row.ProjectXMLID, xmlID: row.XMLID}] = full
	}
	for pi := range sol.Projects.Project {
		projectXMLID := sol.Projects.Project[pi].Id
		for si := range sol.Projects.Project[pi].GraphicScheme {
			stub := sol.Projects.Project[pi].GraphicScheme[si]
			if full, ok := byKey[schemeKey{projectXMLID: projectXMLID, xmlID: stub.Id}]; ok {
				sol.Projects.Project[pi].GraphicScheme[si] = full
			}
		}
	}
	return nil
}
