package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"create_sheme/internal/domain"

	"github.com/google/uuid"
)

func (s *Store) BackfillLeanSolutionData(ctx context.Context) (err error) {
	start := time.Now()
	var processed, skipped int
	defer func() {
		slog.Info("BackfillLeanSolutionData done",
			"processed", processed,
			"skipped", skipped,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("BackfillLeanSolutionData start")

	rows, queryErr := s.pool.Query(ctx, `SELECT id, data FROM solutions`)
	if queryErr != nil {
		err = fmt.Errorf("lean backfill query: %w", queryErr)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var data json.RawMessage
		if scanErr := rows.Scan(&id, &data); scanErr != nil {
			err = fmt.Errorf("lean backfill scan: %w", scanErr)
			return err
		}
		if len(data) == 0 || string(data) == "null" || string(data) == "{}" {
			skipped++
			continue
		}
		var sol domain.Solution
		if unmarshalErr := json.Unmarshal(data, &sol); unmarshalErr != nil {
			err = fmt.Errorf("lean backfill unmarshal solution %s: %w", id, unmarshalErr)
			return err
		}
		needsStrip := sol.HasSchemeGraphicObjects()
		needsMarker := sol.DataLayout != domain.LeanSchemesLayoutV1
		if !needsStrip && !needsMarker {
			skipped++
			continue
		}
		sol.StripSchemeGraphicObjects()
		payload, marshalErr := json.Marshal(sol)
		if marshalErr != nil {
			err = fmt.Errorf("lean backfill marshal solution %s: %w", id, marshalErr)
			return err
		}
		if _, execErr := s.pool.Exec(ctx, `UPDATE solutions SET data=$2, updated_at=NOW() WHERE id=$1`, id, payload); execErr != nil {
			err = fmt.Errorf("lean backfill update solution %s: %w", id, execErr)
			return err
		}
		processed++
	}
	if err = rows.Err(); err != nil {
		err = fmt.Errorf("lean backfill rows: %w", err)
		return err
	}
	return nil
}
