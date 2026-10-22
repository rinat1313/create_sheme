package repository

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type SolutionTypeSchemaRow struct {
	SolutionID uuid.UUID
	Template   string
	Schema     json.RawMessage
	Source     string
	UpdatedAt  time.Time
}

type SolutionObjectRow struct {
	SolutionID         uuid.UUID
	GraphicObjectID    string
	TechnologyObjectID string
	SchemeXMLID        string
	Template           string
	Tag                string
	Name               string
	TargetKeys         []string
	TechnologyPayload  json.RawMessage
}

type SolutionCommandTargetRow struct {
	SolutionID  uuid.UUID
	TargetKey   string
	EntityType  string
	EntityXMLID string
}

type SolutionTemplateParametersRow struct {
	SolutionID    uuid.UUID
	Scope         string
	OwnerTemplate string
	Parameters    []string
}

type SolutionTemplateEntitiesRow struct {
	SolutionID      uuid.UUID
	ObjectTemplate  string
	Scope           string
	EntityTemplates []string
}

func (s *Store) WithSolutionIndexLock(ctx context.Context, solutionID uuid.UUID, fn func(context.Context, Querier) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1013, hashtext($1::text))`, solutionID.String()); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Store) DeleteSolutionIndexes(ctx context.Context, solutionID uuid.UUID) error {
	return s.DeleteSolutionIndexesQ(ctx, s.pool, solutionID)
}

func (s *Store) DeleteSolutionIndexesQ(ctx context.Context, q Querier, solutionID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM solution_template_parameters WHERE solution_id=$1`, solutionID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM solution_template_entities WHERE solution_id=$1`, solutionID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM solution_type_schemas WHERE solution_id=$1`, solutionID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM solution_objects WHERE solution_id=$1`, solutionID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM solution_command_targets WHERE solution_id=$1`, solutionID); err != nil {
		return err
	}
	return s.SetSolutionIndexVersion(ctx, q, solutionID, 0)
}

func (s *Store) BulkInsertTypeSchemas(ctx context.Context, rows []SolutionTypeSchemaRow) error {
	return s.BulkUpsertTypeSchemas(ctx, s.pool, rows)
}

func (s *Store) BulkUpsertTypeSchemas(ctx context.Context, q Querier, rows []SolutionTypeSchemaRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		schema := r.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{}`)
		}
		batch.Queue(`
			INSERT INTO solution_type_schemas (solution_id, template, schema, source)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (solution_id, template) DO UPDATE
			SET schema = EXCLUDED.schema, source = EXCLUDED.source, updated_at = NOW()`,
			r.SolutionID, r.Template, schema, r.Source)
	}
	return s.execBatch(ctx, q, batch, len(rows))
}

func (s *Store) BulkInsertSolutionObjects(ctx context.Context, rows []SolutionObjectRow) error {
	return s.BulkUpsertSolutionObjects(ctx, s.pool, rows)
}

func (s *Store) BulkUpsertSolutionObjects(ctx context.Context, q Querier, rows []SolutionObjectRow) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("BulkUpsertSolutionObjects done",
			"count", len(rows),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("BulkUpsertSolutionObjects start", "count", len(rows))
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		payload := r.TechnologyPayload
		if len(payload) == 0 {
			payload = json.RawMessage(`[]`)
		}
		batch.Queue(`
			INSERT INTO solution_objects
				(solution_id, graphic_object_id, technology_object_id, scheme_xml_id, template, tag, name, target_keys, technology_payload)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (solution_id, graphic_object_id) DO UPDATE
			SET technology_object_id = EXCLUDED.technology_object_id,
				scheme_xml_id = EXCLUDED.scheme_xml_id,
				template = EXCLUDED.template,
				tag = EXCLUDED.tag,
				name = EXCLUDED.name,
				target_keys = EXCLUDED.target_keys,
				technology_payload = EXCLUDED.technology_payload`,
			r.SolutionID, r.GraphicObjectID, r.TechnologyObjectID, r.SchemeXMLID, r.Template, r.Tag, r.Name, r.TargetKeys, payload)
	}
	err = s.execBatch(ctx, q, batch, len(rows))
	return err
}

func (s *Store) BulkInsertCommandTargets(ctx context.Context, rows []SolutionCommandTargetRow) error {
	return s.BulkUpsertCommandTargets(ctx, s.pool, rows)
}

func (s *Store) BulkUpsertCommandTargets(ctx context.Context, q Querier, rows []SolutionCommandTargetRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO solution_command_targets (solution_id, target_key, entity_type, entity_xml_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (solution_id, entity_type, entity_xml_id, target_key) DO NOTHING`,
			r.SolutionID, r.TargetKey, r.EntityType, r.EntityXMLID)
	}
	return s.execBatch(ctx, q, batch, len(rows))
}

func (s *Store) BulkUpsertTemplateParameters(ctx context.Context, q Querier, rows []SolutionTemplateParametersRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		params := r.Parameters
		if params == nil {
			params = []string{}
		}
		batch.Queue(`
			INSERT INTO solution_template_parameters (solution_id, scope, owner_template, parameters)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (solution_id, scope, owner_template) DO UPDATE
			SET parameters = EXCLUDED.parameters`,
			r.SolutionID, r.Scope, r.OwnerTemplate, params)
	}
	return s.execBatch(ctx, q, batch, len(rows))
}

func (s *Store) BulkUpsertTemplateEntities(ctx context.Context, q Querier, rows []SolutionTemplateEntitiesRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		templates := r.EntityTemplates
		if templates == nil {
			templates = []string{}
		}
		batch.Queue(`
			INSERT INTO solution_template_entities (solution_id, object_template, scope, entity_templates)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (solution_id, object_template, scope) DO UPDATE
			SET entity_templates = EXCLUDED.entity_templates`,
			r.SolutionID, r.ObjectTemplate, r.Scope, templates)
	}
	return s.execBatch(ctx, q, batch, len(rows))
}

func (s *Store) execBatch(ctx context.Context, q Querier, batch *pgx.Batch, n int) error {
	type batchSender interface {
		SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	}
	sender, ok := q.(batchSender)
	if !ok {
		return errors.New("querier does not support SendBatch")
	}
	br := sender.SendBatch(ctx, batch)
	defer br.Close()
	for range n {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetTemplateParameters(ctx context.Context, solutionID uuid.UUID, scope, ownerTemplate string) ([]string, error) {
	var params []string
	err := s.pool.QueryRow(ctx, `
		SELECT parameters
		FROM solution_template_parameters
		WHERE solution_id=$1 AND scope=$2 AND owner_template=$3`,
		solutionID, scope, ownerTemplate).Scan(&params)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if params == nil {
		return []string{}, nil
	}
	return params, nil
}

func (s *Store) GetTemplateEntities(ctx context.Context, solutionID uuid.UUID, objectTemplate, scope string) ([]string, error) {
	var templates []string
	err := s.pool.QueryRow(ctx, `
		SELECT entity_templates
		FROM solution_template_entities
		WHERE solution_id=$1 AND object_template=$2 AND scope=$3`,
		solutionID, objectTemplate, scope).Scan(&templates)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if templates == nil {
		return []string{}, nil
	}
	return templates, nil
}

func (s *Store) GetSolutionIndexVersion(ctx context.Context, q Querier, solutionID uuid.UUID) (int, error) {
	if q == nil {
		q = s.pool
	}
	var version int
	err := q.QueryRow(ctx, `SELECT index_version FROM solutions WHERE id=$1`, solutionID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return version, err
}

func (s *Store) SetSolutionIndexVersion(ctx context.Context, q Querier, solutionID uuid.UUID, version int) error {
	if q == nil {
		q = s.pool
	}
	_, err := q.Exec(ctx, `UPDATE solutions SET index_version=$2 WHERE id=$1`, solutionID, version)
	return err
}

func (s *Store) GetSolutionObject(ctx context.Context, solutionID uuid.UUID, graphicObjectID string) (SolutionObjectRow, error) {
	var r SolutionObjectRow
	err := s.pool.QueryRow(ctx, `
		SELECT solution_id, graphic_object_id, scheme_xml_id, template, tag, name, target_keys, technology_payload
		FROM solution_objects
		WHERE solution_id=$1 AND graphic_object_id=$2`,
		solutionID, graphicObjectID).Scan(
		&r.SolutionID, &r.GraphicObjectID, &r.SchemeXMLID, &r.Template, &r.Tag, &r.Name, &r.TargetKeys, &r.TechnologyPayload)
	return r, err
}

func (s *Store) ListSolutionObjectsByScheme(ctx context.Context, solutionID uuid.UUID, schemeXMLID string) ([]SolutionObjectRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT solution_id, graphic_object_id, scheme_xml_id, template, tag, name, target_keys, technology_payload
		FROM solution_objects
		WHERE solution_id=$1 AND scheme_xml_id=$2`,
		solutionID, schemeXMLID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SolutionObjectRow
	for rows.Next() {
		var r SolutionObjectRow
		if err := rows.Scan(&r.SolutionID, &r.GraphicObjectID, &r.SchemeXMLID, &r.Template, &r.Tag, &r.Name, &r.TargetKeys, &r.TechnologyPayload); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

type CommandTargetRef struct {
	EntityType  string
	EntityXMLID string
}

func (s *Store) ListCommandTargetsByKeys(ctx context.Context, solutionID uuid.UUID, targetKeys []string) ([]CommandTargetRef, error) {
	if len(targetKeys) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT entity_type, entity_xml_id
		FROM solution_command_targets
		WHERE solution_id=$1 AND target_key = ANY($2::text[])`,
		solutionID, targetKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CommandTargetRef
	for rows.Next() {
		var r CommandTargetRef
		if err := rows.Scan(&r.EntityType, &r.EntityXMLID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetTypeSchema(ctx context.Context, solutionID uuid.UUID, template string) (SolutionTypeSchemaRow, error) {
	var r SolutionTypeSchemaRow
	err := s.pool.QueryRow(ctx, `
		SELECT solution_id, template, schema, source, updated_at
		FROM solution_type_schemas
		WHERE solution_id=$1 AND template=$2`,
		solutionID, template).Scan(&r.SolutionID, &r.Template, &r.Schema, &r.Source, &r.UpdatedAt)
	return r, err
}

func (s *Store) CountSolutionObjects(ctx context.Context, solutionID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM solution_objects WHERE solution_id=$1`, solutionID).Scan(&n)
	return n, err
}

type SolutionObjectSearchRow struct {
	ObjectID           string
	TechnologyObjectID string
	Name               string
	Tag                string
	ObjectType         string
	SchemeXMLID        string
	SchemeName         string
	ProjectXMLID       string
	ProjectName        string
}

func (s *Store) SearchSolutionObjects(ctx context.Context, solutionID uuid.UUID, query string, idOnly bool, limit, offset int) (result []SolutionObjectSearchRow, total int, err error) {
	start := time.Now()
	defer func() {
		slog.Info("SearchSolutionObjects done",
			"solution_id", solutionID,
			"query", query,
			"id_only", idOnly,
			"total", total,
			"returned", len(result),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("SearchSolutionObjects start",
		"solution_id", solutionID,
		"query", query,
		"id_only", idOnly,
		"limit", limit,
		"offset", offset,
	)

	where := `
		so.solution_id = $1
		AND (
			so.name ILIKE $2
			OR so.tag ILIKE $2
			OR so.graphic_object_id ILIKE $2
			OR so.technology_object_id ILIKE $2
			OR so.template ILIKE $2
		)`
	pattern := "%" + query + "%"
	if idOnly {
		where = `
			so.solution_id = $1
			AND (
				so.graphic_object_id = $2
				OR so.graphic_object_id ILIKE $2 || '%'
				OR so.technology_object_id = $2
				OR so.technology_object_id ILIKE $2 || '%'
			)`
		pattern = query
	}

	countSQL := `
		SELECT COUNT(*)
		FROM (
			SELECT DISTINCT so.graphic_object_id
			FROM solution_objects so
			WHERE ` + where + `
		) t`
	if err = s.pool.QueryRow(ctx, countSQL, solutionID, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := `
		SELECT DISTINCT ON (so.graphic_object_id)
			so.graphic_object_id,
			so.technology_object_id,
			so.name,
			so.tag,
			so.template,
			so.scheme_xml_id,
			COALESCE(gs.name, ''),
			COALESCE(gs.project_xml_id, ''),
			COALESCE((
				SELECT pn.name
				FROM solution_nodes pn
				WHERE pn.solution_id = so.solution_id
					AND pn.xml_id = gs.project_xml_id
					AND pn.node_type = 'project'
				ORDER BY pn.sort_index
				LIMIT 1
			), '')
		FROM solution_objects so
		LEFT JOIN graphic_schemes gs
			ON gs.solution_id = so.solution_id AND gs.xml_id = so.scheme_xml_id
		WHERE ` + where + `
		ORDER BY so.graphic_object_id, so.name
		LIMIT $3 OFFSET $4`

	rows, queryErr := s.pool.Query(ctx, listSQL, solutionID, pattern, limit, offset)
	if queryErr != nil {
		err = queryErr
		return nil, 0, err
	}
	defer rows.Close()

	result = make([]SolutionObjectSearchRow, 0)
	for rows.Next() {
		var r SolutionObjectSearchRow
		if scanErr := rows.Scan(
			&r.ObjectID, &r.TechnologyObjectID, &r.Name, &r.Tag, &r.ObjectType,
			&r.SchemeXMLID, &r.SchemeName, &r.ProjectXMLID, &r.ProjectName,
		); scanErr != nil {
			err = scanErr
			return nil, 0, err
		}
		result = append(result, r)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}
