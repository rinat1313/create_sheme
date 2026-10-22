package repository

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"time"

	"create_sheme/internal/domain"

	"github.com/google/uuid"
)

func (s *Store) BackfillSolutionDataFromXML(ctx context.Context) (err error) {
	start := time.Now()
	var processed, skipped int
	defer func() {
		slog.Info("BackfillSolutionDataFromXML done",
			"processed", processed,
			"skipped", skipped,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("BackfillSolutionDataFromXML start")

	var hasXMLColumns bool
	if err = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'solutions' AND column_name = 'original_xml'
		)`).Scan(&hasXMLColumns); err != nil {
		err = fmt.Errorf("backfill column check: %w", err)
		return err
	}
	if !hasXMLColumns {
		slog.Info("BackfillSolutionDataFromXML skip", "reason", "xml columns already dropped")
		return nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id,
			CASE
				WHEN current_xml IS NOT NULL AND octet_length(current_xml) > 0 THEN current_xml
				ELSE original_xml
			END AS active_xml,
			data
		FROM solutions`)
	if err != nil {
		return fmt.Errorf("backfill query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var activeXML []byte
		var data json.RawMessage
		if scanErr := rows.Scan(&id, &activeXML, &data); scanErr != nil {
			err = fmt.Errorf("backfill scan: %w", scanErr)
			return err
		}
		if len(data) > 0 && string(data) != "null" && string(data) != "{}" {
			skipped++
			continue
		}
		payload := json.RawMessage(`{}`)
		if len(activeXML) > 0 {
			var sol domain.Solution
			if unmarshalErr := xml.Unmarshal(activeXML, &sol); unmarshalErr != nil {
				err = fmt.Errorf("backfill unmarshal solution %s: %w", id, unmarshalErr)
				return err
			}
			marshaled, marshalErr := json.Marshal(sol)
			if marshalErr != nil {
				err = fmt.Errorf("backfill marshal solution %s: %w", id, marshalErr)
				return err
			}
			payload = marshaled
		}
		if _, execErr := s.pool.Exec(ctx, `UPDATE solutions SET data=$2, updated_at=NOW() WHERE id=$1`, id, payload); execErr != nil {
			err = fmt.Errorf("backfill update solution %s: %w", id, execErr)
			return err
		}
		processed++
	}
	if err = rows.Err(); err != nil {
		err = fmt.Errorf("backfill rows: %w", err)
		return err
	}
	return nil
}
