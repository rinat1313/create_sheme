package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ComplianceDuplicateScanner struct {
	SolutionID   uuid.UUID  `json:"solution_id"`
	Enabled      bool       `json:"enabled"`
	Mode         string     `json:"mode"`
	CursorOffset int        `json:"cursor_offset"`
	Processed    int        `json:"processed"`
	Total        int        `json:"total"`
	RuleID       *uuid.UUID `json:"rule_id,omitempty"`
	SessionID    *uuid.UUID `json:"session_id,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ComplianceDuplicateRow struct {
	SolutionID         uuid.UUID `json:"solution_id"`
	Tag                string    `json:"tag"`
	ObjectID           string    `json:"object_id"`
	TechnologyObjectID string    `json:"technology_object_id"`
	ObjectName         string    `json:"object_name"`
	ObjectType         string    `json:"object_type"`
	SchemeXMLID        string    `json:"scheme_xml_id"`
	SchemeName         string    `json:"scheme_name"`
	Scope              string    `json:"scope"`
	ScopeLabel         string    `json:"scope_label"`
	SourceLabel        string    `json:"source_label"`
	PropertyID         string    `json:"property_id"`
	EntityID           string    `json:"entity_id"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ComplianceDuplicateDirty struct {
	SolutionID uuid.UUID `json:"solution_id"`
	ObjectID   string    `json:"object_id"`
	Priority   int       `json:"priority"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Store) UpsertComplianceDuplicateScanner(ctx context.Context, sc ComplianceDuplicateScanner) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_duplicate_scanner (
			solution_id, enabled, mode, cursor_offset, processed, total, rule_id, session_id, status, error_message, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		ON CONFLICT (solution_id) DO UPDATE SET
			enabled=EXCLUDED.enabled,
			mode=EXCLUDED.mode,
			cursor_offset=EXCLUDED.cursor_offset,
			processed=EXCLUDED.processed,
			total=EXCLUDED.total,
			rule_id=EXCLUDED.rule_id,
			session_id=EXCLUDED.session_id,
			status=EXCLUDED.status,
			error_message=EXCLUDED.error_message,
			updated_at=NOW()`,
		sc.SolutionID, sc.Enabled, sc.Mode, sc.CursorOffset, sc.Processed, sc.Total, sc.RuleID, sc.SessionID, sc.Status, sc.ErrorMessage)
	return err
}

func (s *Store) GetComplianceDuplicateScanner(ctx context.Context, solutionID uuid.UUID) (ComplianceDuplicateScanner, error) {
	var sc ComplianceDuplicateScanner
	err := s.pool.QueryRow(ctx, `
		SELECT solution_id, enabled, mode, cursor_offset, processed, total, rule_id, session_id, status, error_message, created_at, updated_at
		FROM compliance_duplicate_scanner WHERE solution_id=$1`, solutionID).Scan(
		&sc.SolutionID, &sc.Enabled, &sc.Mode, &sc.CursorOffset, &sc.Processed, &sc.Total,
		&sc.RuleID, &sc.SessionID, &sc.Status, &sc.ErrorMessage, &sc.CreatedAt, &sc.UpdatedAt)
	return sc, err
}

func (s *Store) ListEnabledComplianceDuplicateScanners(ctx context.Context) ([]ComplianceDuplicateScanner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT solution_id, enabled, mode, cursor_offset, processed, total, rule_id, session_id, status, error_message, created_at, updated_at
		FROM compliance_duplicate_scanner
		WHERE enabled=TRUE AND rule_id IS NOT NULL
		ORDER BY updated_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComplianceDuplicateScanner
	for rows.Next() {
		var sc ComplianceDuplicateScanner
		if err := rows.Scan(
			&sc.SolutionID, &sc.Enabled, &sc.Mode, &sc.CursorOffset, &sc.Processed, &sc.Total,
			&sc.RuleID, &sc.SessionID, &sc.Status, &sc.ErrorMessage, &sc.CreatedAt, &sc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) ListAllComplianceTagObjectTags(ctx context.Context, solutionID uuid.UUID) (map[string]map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT object_id, tag, contrib FROM compliance_tag_object_tags WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]int{}
	for rows.Next() {
		var objectID, tag string
		var contrib int
		if err := rows.Scan(&objectID, &tag, &contrib); err != nil {
			return nil, err
		}
		m := out[objectID]
		if m == nil {
			m = map[string]int{}
			out[objectID] = m
		}
		m[tag] = contrib
	}
	return out, rows.Err()
}

func (s *Store) UpdateComplianceDuplicateScannerProgress(ctx context.Context, solutionID uuid.UUID, mode string, cursor, processed, total int, status, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE compliance_duplicate_scanner
		SET mode=$2, cursor_offset=$3, processed=$4, total=$5, status=$6, error_message=$7, updated_at=NOW()
		WHERE solution_id=$1`,
		solutionID, mode, cursor, processed, total, status, errMsg)
	return err
}

func (s *Store) SetComplianceDuplicateScannerEnabled(ctx context.Context, solutionID uuid.UUID, enabled bool, ruleID, sessionID *uuid.UUID, mode, status string) error {
	if enabled {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO compliance_duplicate_scanner (
				solution_id, enabled, mode, cursor_offset, processed, total, rule_id, session_id, status, error_message, updated_at
			) VALUES ($1,TRUE,$2,0,0,0,$3,$4,$5,'',NOW())
			ON CONFLICT (solution_id) DO UPDATE SET
				enabled=TRUE,
				mode=EXCLUDED.mode,
				cursor_offset=0,
				processed=0,
				total=0,
				rule_id=EXCLUDED.rule_id,
				session_id=EXCLUDED.session_id,
				status=EXCLUDED.status,
				error_message='',
				updated_at=NOW()`,
			solutionID, mode, ruleID, sessionID, status)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_duplicate_scanner (
			solution_id, enabled, mode, cursor_offset, processed, total, rule_id, session_id, status, error_message, updated_at
		) VALUES ($1,FALSE,'idle',0,0,0,$2,$3,'idle','',NOW())
		ON CONFLICT (solution_id) DO UPDATE SET
			enabled=FALSE,
			mode='idle',
			status='idle',
			session_id=EXCLUDED.session_id,
			error_message='',
			updated_at=NOW()`,
		solutionID, ruleID, sessionID)
	return err
}

func (s *Store) ClearComplianceTagCounts(ctx context.Context, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_tag_counts WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) ClearComplianceTagObjectTags(ctx context.Context, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_tag_object_tags WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) ClearComplianceDuplicateRows(ctx context.Context, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_duplicate_rows WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) ClearComplianceDuplicateDirty(ctx context.Context, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_duplicate_dirty WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) UpsertComplianceTagCount(ctx context.Context, solutionID uuid.UUID, tag string, count int) error {
	if count <= 0 {
		_, err := s.pool.Exec(ctx, `DELETE FROM compliance_tag_counts WHERE solution_id=$1 AND tag=$2`, solutionID, tag)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_tag_counts (solution_id, tag, count) VALUES ($1,$2,$3)
		ON CONFLICT (solution_id, tag) DO UPDATE SET count=EXCLUDED.count`,
		solutionID, tag, count)
	return err
}

func (s *Store) GetComplianceTagCount(ctx context.Context, solutionID uuid.UUID, tag string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count FROM compliance_tag_counts WHERE solution_id=$1 AND tag=$2`, solutionID, tag).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func (s *Store) ListComplianceTagCounts(ctx context.Context, solutionID uuid.UUID) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT tag, count FROM compliance_tag_counts WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		out[tag] = count
	}
	return out, rows.Err()
}

func (s *Store) ListComplianceTagObjectTags(ctx context.Context, solutionID uuid.UUID, objectID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tag, contrib FROM compliance_tag_object_tags WHERE solution_id=$1 AND object_id=$2`, solutionID, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var tag string
		var contrib int
		if err := rows.Scan(&tag, &contrib); err != nil {
			return nil, err
		}
		out[tag] = contrib
	}
	return out, rows.Err()
}

func (s *Store) ReplaceComplianceTagObjectTags(ctx context.Context, solutionID uuid.UUID, objectID string, tagContrib map[string]int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM compliance_tag_object_tags WHERE solution_id=$1 AND object_id=$2`, solutionID, objectID); err != nil {
		return err
	}
	for tag, contrib := range tagContrib {
		if contrib <= 0 || tag == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO compliance_tag_object_tags (solution_id, object_id, tag, contrib) VALUES ($1,$2,$3,$4)`,
			solutionID, objectID, tag, contrib); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ObjectHasComplianceDuplicateRows(ctx context.Context, solutionID uuid.UUID, objectID string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM compliance_duplicate_rows WHERE solution_id=$1 AND object_id=$2`, solutionID, objectID).Scan(&n)
	return n > 0, err
}

func (s *Store) DeleteComplianceDuplicateRowsForTag(ctx context.Context, solutionID uuid.UUID, tag string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_duplicate_rows WHERE solution_id=$1 AND tag=$2`, solutionID, tag)
	return err
}

func (s *Store) UpsertComplianceDuplicateRow(ctx context.Context, row ComplianceDuplicateRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_duplicate_rows (
			solution_id, tag, object_id, object_name, object_type, scheme_xml_id, scheme_name,
			scope, scope_label, source_label, property_id, entity_id, technology_object_id, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (solution_id, tag, object_id, scope, property_id, entity_id, source_label) DO UPDATE SET
			object_name=EXCLUDED.object_name,
			object_type=EXCLUDED.object_type,
			scheme_xml_id=EXCLUDED.scheme_xml_id,
			scheme_name=EXCLUDED.scheme_name,
			scope_label=EXCLUDED.scope_label,
			technology_object_id=EXCLUDED.technology_object_id,
			updated_at=NOW()`,
		row.SolutionID, row.Tag, row.ObjectID, row.ObjectName, row.ObjectType, row.SchemeXMLID, row.SchemeName,
		row.Scope, row.ScopeLabel, row.SourceLabel, row.PropertyID, row.EntityID, row.TechnologyObjectID)
	return err
}

func (s *Store) ListComplianceDuplicateRows(ctx context.Context, solutionID uuid.UUID) ([]ComplianceDuplicateRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.solution_id, r.tag, r.object_id, r.object_name, r.object_type, r.scheme_xml_id, r.scheme_name,
			r.scope, r.scope_label, r.source_label, r.property_id, r.entity_id, r.updated_at,
			COALESCE(NULLIF(TRIM(r.technology_object_id), ''), so.technology_object_id, '') AS technology_object_id
		FROM compliance_duplicate_rows r
		LEFT JOIN solution_objects so
			ON so.solution_id = r.solution_id AND so.graphic_object_id = r.object_id
		WHERE r.solution_id=$1
		ORDER BY r.tag, r.object_id, r.source_label`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComplianceDuplicateRow
	for rows.Next() {
		var row ComplianceDuplicateRow
		if err := rows.Scan(
			&row.SolutionID, &row.Tag, &row.ObjectID, &row.ObjectName, &row.ObjectType,
			&row.SchemeXMLID, &row.SchemeName, &row.Scope, &row.ScopeLabel, &row.SourceLabel,
			&row.PropertyID, &row.EntityID, &row.UpdatedAt, &row.TechnologyObjectID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListDuplicateTags(ctx context.Context, solutionID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tag FROM compliance_tag_counts WHERE solution_id=$1 AND count >= 2 ORDER BY tag`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

func (s *Store) IsDuplicateTag(ctx context.Context, solutionID uuid.UUID, tag string) (bool, error) {
	count, err := s.GetComplianceTagCount(ctx, solutionID, tag)
	if err != nil {
		return false, err
	}
	return count >= 2, nil
}

func (s *Store) EnqueueComplianceDuplicateDirty(ctx context.Context, solutionID uuid.UUID, objectID string, priority int, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_duplicate_dirty (solution_id, object_id, priority, reason, created_at)
		VALUES ($1,$2,$3,$4,NOW())
		ON CONFLICT (solution_id, object_id) DO UPDATE SET
			priority=GREATEST(compliance_duplicate_dirty.priority, EXCLUDED.priority),
			reason=EXCLUDED.reason,
			created_at=CASE
				WHEN EXCLUDED.priority > compliance_duplicate_dirty.priority THEN NOW()
				ELSE compliance_duplicate_dirty.created_at
			END`,
		solutionID, objectID, priority, reason)
	return err
}

func (s *Store) PopComplianceDuplicateDirty(ctx context.Context, solutionID uuid.UUID, limit int) ([]ComplianceDuplicateDirty, error) {
	if limit < 1 {
		limit = 1
	}
	rows, err := s.pool.Query(ctx, `
		DELETE FROM compliance_duplicate_dirty
		WHERE ctid IN (
			SELECT ctid FROM compliance_duplicate_dirty
			WHERE solution_id=$1
			ORDER BY priority DESC, created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING solution_id, object_id, priority, reason, created_at`, solutionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComplianceDuplicateDirty
	for rows.Next() {
		var d ComplianceDuplicateDirty
		if err := rows.Scan(&d.SolutionID, &d.ObjectID, &d.Priority, &d.Reason, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) CountComplianceDuplicateDirty(ctx context.Context, solutionID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM compliance_duplicate_dirty WHERE solution_id=$1`, solutionID).Scan(&n)
	return n, err
}

func (s *Store) HasActiveComplianceDuplicateJob(ctx context.Context, solutionID uuid.UUID) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM compliance_duplicate_jobs
		WHERE solution_id=$1 AND status IN ('pending', 'running')`, solutionID).Scan(&n)
	return n > 0, err
}
