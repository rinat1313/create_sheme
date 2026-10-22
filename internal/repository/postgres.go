package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SolutionRow struct {
	ID           uuid.UUID
	Name         string
	Company      string
	Description  string
	ExternalID   string
	Data         json.RawMessage
	ImportStatus string
	ImportError  string
	IsDirty      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SolutionSummary struct {
	ID           uuid.UUID
	Name         string
	Company      string
	Description  string
	ImportStatus string
	ImportError  string
	IsDirty      bool
	CreatedAt    time.Time
}

type NodeRow struct {
	ID         uuid.UUID
	SolutionID uuid.UUID
	ParentID   *uuid.UUID
	NodeType   string
	XMLID      string
	Name       string
	Tag        string
	SortIndex  int
	Meta       json.RawMessage
}

type SchemeRow struct {
	ID           uuid.UUID
	SolutionID   uuid.UUID
	XMLID        string
	ProjectXMLID string
	Name         string
	Template     string
	Payload      json.RawMessage
}

type DisplaySetting struct {
	Template      string          `json:"template"`
	Renderer      string          `json:"renderer"`
	VisibleFields json.RawMessage `json:"visible_fields"`
	Style         json.RawMessage `json:"style"`
}

type ChangeRow struct {
	ID          uuid.UUID
	SchemeXMLID string
	EntityType  string
	EntityXMLID string
	Patch       json.RawMessage
}

type SolutionMetadataRow struct {
	SolutionID      uuid.UUID
	Automations     json.RawMessage
	ControlCommands json.RawMessage
}

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}

func (s *Store) CreateSolution(ctx context.Context, row SolutionRow) error {
	data := row.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO solutions (id, name, company, description, external_id, data, import_status, import_error, is_dirty)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		row.ID, row.Name, row.Company, row.Description, row.ExternalID, data, row.ImportStatus, row.ImportError, row.IsDirty)
	return err
}

func (s *Store) DeleteSolution(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM solutions WHERE id=$1`, id)
	return err
}

func (s *Store) UpdateImportStatus(ctx context.Context, id uuid.UUID, status, importError string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE solutions SET import_status=$2, import_error=$3, updated_at=NOW() WHERE id=$1`,
		id, status, importError)
	return err
}

func (s *Store) UpdateSolutionImportMeta(ctx context.Context, id uuid.UUID, name, company, description, externalID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE solutions
		SET name=$2, company=$3, description=$4, external_id=$5, import_error='', updated_at=NOW()
		WHERE id=$1`,
		id, name, company, description, externalID)
	return err
}

func (s *Store) ClearSolutionContent(ctx context.Context, id uuid.UUID) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM solution_nodes WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM graphic_schemes WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM solution_templates WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM solution_metadata WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if err := s.DeleteSolutionIndexes(ctx, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM edit_sessions WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM working_changes WHERE solution_id=$1`, id); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE solutions
		SET data='{}'::jsonb, data_blob=NULL, data_encoding=$2, is_dirty=FALSE, index_version=0, updated_at=NOW()
		WHERE id=$1`, id, EncodingJSONB); err != nil {
		return err
	}
	return nil
}

func (s *Store) UpdateSolutionData(ctx context.Context, id uuid.UUID, data json.RawMessage, isDirty bool) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("UpdateSolutionData done",
			"solution_id", id,
			"raw_bytes", len(data),
			"is_dirty", isDirty,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("UpdateSolutionData start", "solution_id", id, "raw_bytes", len(data), "is_dirty", isDirty)
	if len(data) == 0 {
		data = json.RawMessage(jsonbStub)
	}
	blob, encoding, encErr := EncodeGzipJSON(data)
	if encErr != nil {
		err = encErr
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE solutions
		SET data=$2::jsonb, data_blob=$3, data_encoding=$4, is_dirty=$5, updated_at=NOW()
		WHERE id=$1`,
		id, jsonbStub, blob, encoding, isDirty)
	return err
}

func (s *Store) ListSolutionsSummary(ctx context.Context) ([]SolutionSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, company, description, import_status, import_error, is_dirty, created_at
		FROM solutions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SolutionSummary
	for rows.Next() {
		var r SolutionSummary
		if err := rows.Scan(&r.ID, &r.Name, &r.Company, &r.Description, &r.ImportStatus, &r.ImportError, &r.IsDirty, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetSolutionMeta(ctx context.Context, id uuid.UUID) (SolutionSummary, error) {
	var r SolutionSummary
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, company, description, import_status, import_error, is_dirty, created_at
		FROM solutions WHERE id=$1`, id).Scan(
		&r.ID, &r.Name, &r.Company, &r.Description, &r.ImportStatus, &r.ImportError, &r.IsDirty, &r.CreatedAt)
	return r, err
}

func (s *Store) DeleteAllSolutions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM solutions`)
	return err
}

func (s *Store) GetSolution(ctx context.Context, id uuid.UUID) (SolutionRow, error) {
	var r SolutionRow
	var dataJSONB json.RawMessage
	var dataBlob []byte
	var dataEncoding string
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, company, description, external_id, data, data_blob, data_encoding,
			import_status, import_error, is_dirty, created_at, updated_at
		FROM solutions WHERE id=$1`, id).Scan(
		&r.ID, &r.Name, &r.Company, &r.Description, &r.ExternalID, &dataJSONB, &dataBlob, &dataEncoding,
		&r.ImportStatus, &r.ImportError, &r.IsDirty, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	decoded, decodeErr := DecodeStoredJSON(dataEncoding, dataJSONB, dataBlob)
	if decodeErr != nil {
		return r, decodeErr
	}
	r.Data = decoded
	return r, nil
}

func (s *Store) GetSolutionData(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	start := time.Now()
	var dataJSONB json.RawMessage
	var dataBlob []byte
	var dataEncoding string
	err := s.pool.QueryRow(ctx, `
		SELECT data, data_blob, data_encoding FROM solutions WHERE id=$1`, id).Scan(
		&dataJSONB, &dataBlob, &dataEncoding)
	if err != nil {
		return nil, err
	}
	raw, decodeErr := DecodeStoredJSON(dataEncoding, dataJSONB, dataBlob)
	slog.Debug("GetSolutionData done",
		"solution_id", id,
		"encoding", dataEncoding,
		"raw_bytes", len(raw),
		"duration_ms", time.Since(start).Milliseconds(),
		"err", decodeErr,
	)
	return raw, decodeErr
}

func (s *Store) InsertNode(ctx context.Context, n NodeRow) error {
	meta := n.Meta
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO solution_nodes (id, solution_id, parent_id, node_type, xml_id, name, tag, sort_index, meta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		n.ID, n.SolutionID, n.ParentID, n.NodeType, n.XMLID, n.Name, n.Tag, n.SortIndex, meta)
	return err
}

func (s *Store) ListNodes(ctx context.Context, solutionID uuid.UUID) ([]NodeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, solution_id, parent_id, node_type, xml_id, name, tag, sort_index, meta
		FROM solution_nodes WHERE solution_id=$1 ORDER BY sort_index, name`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []NodeRow
	for rows.Next() {
		var r NodeRow
		if err := rows.Scan(&r.ID, &r.SolutionID, &r.ParentID, &r.NodeType, &r.XMLID, &r.Name, &r.Tag, &r.SortIndex, &r.Meta); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) InsertScheme(ctx context.Context, sch SchemeRow) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("InsertScheme done",
			"solution_id", sch.SolutionID,
			"scheme_xml_id", sch.XMLID,
			"raw_bytes", len(sch.Payload),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("InsertScheme start", "solution_id", sch.SolutionID, "scheme_xml_id", sch.XMLID, "raw_bytes", len(sch.Payload))
	blob, encoding, encErr := EncodeGzipJSON(sch.Payload)
	if encErr != nil {
		err = encErr
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO graphic_schemes (id, solution_id, xml_id, project_xml_id, name, template, payload, payload_blob, payload_encoding)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`,
		sch.ID, sch.SolutionID, sch.XMLID, sch.ProjectXMLID, sch.Name, sch.Template, jsonbStub, blob, encoding)
	return err
}

func (s *Store) UpdateSchemePayload(ctx context.Context, solutionID uuid.UUID, schemeXMLID string, payload json.RawMessage) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("UpdateSchemePayload done",
			"solution_id", solutionID,
			"scheme_xml_id", schemeXMLID,
			"raw_bytes", len(payload),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("UpdateSchemePayload start", "solution_id", solutionID, "scheme_xml_id", schemeXMLID, "raw_bytes", len(payload))
	blob, encoding, encErr := EncodeGzipJSON(payload)
	if encErr != nil {
		err = encErr
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE graphic_schemes
		SET payload=$3::jsonb, payload_blob=$4, payload_encoding=$5, updated_at=NOW()
		WHERE solution_id=$1 AND xml_id=$2`,
		solutionID, schemeXMLID, jsonbStub, blob, encoding)
	return err
}

func scanSchemeRow(id, solutionID uuid.UUID, xmlID, projectXMLID, name, template string, payloadJSONB json.RawMessage, payloadBlob []byte, payloadEncoding string) (SchemeRow, error) {
	raw, err := DecodeStoredJSON(payloadEncoding, payloadJSONB, payloadBlob)
	if err != nil {
		return SchemeRow{}, err
	}
	return SchemeRow{
		ID:           id,
		SolutionID:   solutionID,
		XMLID:        xmlID,
		ProjectXMLID: projectXMLID,
		Name:         name,
		Template:     template,
		Payload:      raw,
	}, nil
}

func (s *Store) GetScheme(ctx context.Context, solutionID uuid.UUID, schemeXMLID string) (SchemeRow, error) {
	start := time.Now()
	var id uuid.UUID
	var projectXMLID, name, template, payloadEncoding string
	var payloadJSONB json.RawMessage
	var payloadBlob []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, solution_id, xml_id, project_xml_id, name, template, payload, payload_blob, payload_encoding
		FROM graphic_schemes WHERE solution_id=$1 AND xml_id=$2`, solutionID, schemeXMLID).Scan(
		&id, &solutionID, &schemeXMLID, &projectXMLID, &name, &template, &payloadJSONB, &payloadBlob, &payloadEncoding)
	if err != nil {
		return SchemeRow{}, err
	}
	row, decodeErr := scanSchemeRow(id, solutionID, schemeXMLID, projectXMLID, name, template, payloadJSONB, payloadBlob, payloadEncoding)
	slog.Debug("GetScheme done",
		"solution_id", solutionID,
		"scheme_xml_id", schemeXMLID,
		"encoding", payloadEncoding,
		"raw_bytes", len(row.Payload),
		"duration_ms", time.Since(start).Milliseconds(),
		"err", decodeErr,
	)
	return row, decodeErr
}

func (s *Store) ListSchemes(ctx context.Context, solutionID uuid.UUID) ([]SchemeRow, error) {
	start := time.Now()
	rows, err := s.pool.Query(ctx, `
		SELECT id, solution_id, xml_id, project_xml_id, name, template, payload, payload_blob, payload_encoding
		FROM graphic_schemes WHERE solution_id=$1 ORDER BY name`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SchemeRow
	for rows.Next() {
		var id, solID uuid.UUID
		var xmlID, projectXMLID, name, template, payloadEncoding string
		var payloadJSONB json.RawMessage
		var payloadBlob []byte
		if scanErr := rows.Scan(&id, &solID, &xmlID, &projectXMLID, &name, &template, &payloadJSONB, &payloadBlob, &payloadEncoding); scanErr != nil {
			return nil, scanErr
		}
		row, decodeErr := scanSchemeRow(id, solID, xmlID, projectXMLID, name, template, payloadJSONB, payloadBlob, payloadEncoding)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slog.Debug("ListSchemes done",
		"solution_id", solutionID,
		"count", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return result, nil
}

func (s *Store) CreateSession(ctx context.Context, id, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO edit_sessions (id, solution_id) VALUES ($1,$2)`, id, solutionID)
	return err
}

func (s *Store) SetActiveScheme(ctx context.Context, sessionID uuid.UUID, schemeXMLID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE edit_sessions SET active_scheme_xml_id=$2 WHERE id=$1`, sessionID, schemeXMLID)
	return err
}

func (s *Store) GetSession(ctx context.Context, sessionID uuid.UUID) (uuid.UUID, string, error) {
	var solutionID uuid.UUID
	var schemeXMLID string
	err := s.pool.QueryRow(ctx, `SELECT solution_id, active_scheme_xml_id FROM edit_sessions WHERE id=$1`, sessionID).Scan(&solutionID, &schemeXMLID)
	return solutionID, schemeXMLID, err
}

func (s *Store) ListSessionIDs(ctx context.Context, solutionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM edit_sessions WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) UpsertDraftChange(ctx context.Context, id, sessionID uuid.UUID, schemeXMLID, entityType, entityXMLID string, patch json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM draft_changes WHERE session_id=$1 AND entity_type=$2 AND entity_xml_id=$3`,
		sessionID, entityType, entityXMLID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO draft_changes (id, session_id, scheme_xml_id, entity_type, entity_xml_id, patch)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		id, sessionID, schemeXMLID, entityType, entityXMLID, patch)
	if err != nil {
		return err
	}
	if entityType == "graphic_object" {
		solutionID, _, sessionErr := s.GetSession(ctx, sessionID)
		if sessionErr != nil {
			return sessionErr
		}
		if delErr := s.DeleteExportChangeAck(ctx, solutionID, entityType, entityXMLID); delErr != nil {
			return delErr
		}
	}
	return nil
}

func (s *Store) ListDraftChanges(ctx context.Context, sessionID uuid.UUID) ([]ChangeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, scheme_xml_id, entity_type, entity_xml_id, patch FROM draft_changes WHERE session_id=$1`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ChangeRow
	for rows.Next() {
		var r ChangeRow
		if err := rows.Scan(&r.ID, &r.SchemeXMLID, &r.EntityType, &r.EntityXMLID, &r.Patch); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ClearDraftChanges(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM draft_changes WHERE session_id=$1`, sessionID)
	return err
}

func (s *Store) ClearDraftChangesExceptScheme(ctx context.Context, sessionID uuid.UUID, schemeXMLID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM draft_changes WHERE session_id=$1 AND scheme_xml_id<>$2`, sessionID, schemeXMLID)
	return err
}

func (s *Store) FlushDraftToWorking(ctx context.Context, solutionID, sessionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("FlushDraftToWorking done",
			"solution_id", solutionID,
			"session_id", sessionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("FlushDraftToWorking start", "solution_id", solutionID, "session_id", sessionID)

	var changes []ChangeRow
	changes, err = s.ListDraftChanges(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, ch := range changes {
		if err = s.UpsertWorkingChange(ctx, solutionID, ch.SchemeXMLID, ch.EntityType, ch.EntityXMLID, ch.Patch); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `UPDATE solutions SET is_dirty=TRUE, updated_at=NOW() WHERE id=$1`, solutionID)
	if err != nil {
		return err
	}
	return s.ClearDraftChanges(ctx, sessionID)
}

func (s *Store) ListWorkingChanges(ctx context.Context, solutionID uuid.UUID) ([]ChangeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, scheme_xml_id, entity_type, entity_xml_id, patch FROM working_changes WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ChangeRow
	for rows.Next() {
		var r ChangeRow
		if err := rows.Scan(&r.ID, &r.SchemeXMLID, &r.EntityType, &r.EntityXMLID, &r.Patch); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetSolutionMetadata(ctx context.Context, solutionID uuid.UUID) (SolutionMetadataRow, error) {
	var r SolutionMetadataRow
	err := s.pool.QueryRow(ctx, `
		SELECT solution_id, automations, control_commands
		FROM solution_metadata WHERE solution_id=$1`, solutionID).Scan(
		&r.SolutionID, &r.Automations, &r.ControlCommands)
	return r, err
}

func (s *Store) UpsertSolutionMetadata(ctx context.Context, solutionID uuid.UUID, automations, controlCommands json.RawMessage) error {
	if len(automations) == 0 {
		automations = json.RawMessage(`[]`)
	}
	if len(controlCommands) == 0 {
		controlCommands = json.RawMessage(`[]`)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO solution_metadata (solution_id, automations, control_commands)
		VALUES ($1, $2, $3)
		ON CONFLICT (solution_id) DO UPDATE SET
			automations = EXCLUDED.automations,
			control_commands = EXCLUDED.control_commands,
			updated_at = NOW()`,
		solutionID, automations, controlCommands)
	return err
}

func (s *Store) ListWorkingChangesForScheme(ctx context.Context, solutionID uuid.UUID, schemeXMLID string) ([]ChangeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, scheme_xml_id, entity_type, entity_xml_id, patch
		FROM working_changes
		WHERE solution_id=$1 AND scheme_xml_id=$2 AND entity_type='graphic_object'`,
		solutionID, schemeXMLID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ChangeRow
	for rows.Next() {
		var r ChangeRow
		if err := rows.Scan(&r.ID, &r.SchemeXMLID, &r.EntityType, &r.EntityXMLID, &r.Patch); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ListMetadataWorkingChanges(ctx context.Context, solutionID uuid.UUID) ([]ChangeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, scheme_xml_id, entity_type, entity_xml_id, patch
		FROM working_changes
		WHERE solution_id=$1 AND entity_type IN ('control_command', 'automation')`,
		solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ChangeRow
	for rows.Next() {
		var r ChangeRow
		if err := rows.Scan(&r.ID, &r.SchemeXMLID, &r.EntityType, &r.EntityXMLID, &r.Patch); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ClearWorkingChanges(ctx context.Context, solutionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("ClearWorkingChanges done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("ClearWorkingChanges start", "solution_id", solutionID)
	_, err = s.pool.Exec(ctx, `DELETE FROM working_changes WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) UpsertWorkingChange(ctx context.Context, solutionID uuid.UUID, schemeXMLID, entityType, entityXMLID string, patch json.RawMessage) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("UpsertWorkingChange done",
			"solution_id", solutionID,
			"entity_type", entityType,
			"entity_xml_id", entityXMLID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("UpsertWorkingChange start",
		"solution_id", solutionID,
		"entity_type", entityType,
		"entity_xml_id", entityXMLID,
	)
	_, err = s.pool.Exec(ctx, `
		DELETE FROM working_changes WHERE solution_id=$1 AND entity_type=$2 AND entity_xml_id=$3`,
		solutionID, entityType, entityXMLID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO working_changes (id, solution_id, scheme_xml_id, entity_type, entity_xml_id, patch)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		uuid.New(), solutionID, schemeXMLID, entityType, entityXMLID, patch)
	if err != nil {
		return err
	}
	if entityType == "graphic_object" {
		if err = s.UpsertSolutionChangedObject(ctx, solutionID, entityType, entityXMLID, schemeXMLID); err != nil {
			return err
		}
		if err = s.DeleteExportChangeAck(ctx, solutionID, entityType, entityXMLID); err != nil {
			return err
		}
	}
	return nil
}

type SolutionChangedObjectRow struct {
	EntityType  string
	EntityXMLID string
	SchemeXMLID string
	RecordedAt  time.Time
}

func (s *Store) UpsertSolutionChangedObject(ctx context.Context, solutionID uuid.UUID, entityType, entityXMLID, schemeXMLID string) (err error) {
	start := time.Now()
	defer func() {
		slog.Debug("UpsertSolutionChangedObject done",
			"solution_id", solutionID,
			"entity_type", entityType,
			"entity_xml_id", entityXMLID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("UpsertSolutionChangedObject start",
		"solution_id", solutionID,
		"entity_type", entityType,
		"entity_xml_id", entityXMLID,
	)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO solution_changed_objects (solution_id, entity_type, entity_xml_id, scheme_xml_id, recorded_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (solution_id, entity_type, entity_xml_id) DO UPDATE SET
			scheme_xml_id = EXCLUDED.scheme_xml_id,
			recorded_at = NOW()`,
		solutionID, entityType, entityXMLID, schemeXMLID)
	return err
}

func (s *Store) ListSolutionChangedObjects(ctx context.Context, solutionID uuid.UUID) (result []SolutionChangedObjectRow, err error) {
	start := time.Now()
	defer func() {
		slog.Debug("ListSolutionChangedObjects done",
			"solution_id", solutionID,
			"count", len(result),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("ListSolutionChangedObjects start", "solution_id", solutionID)
	rows, err := s.pool.Query(ctx, `
		SELECT entity_type, entity_xml_id, scheme_xml_id, recorded_at
		FROM solution_changed_objects WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r SolutionChangedObjectRow
		if err = rows.Scan(&r.EntityType, &r.EntityXMLID, &r.SchemeXMLID, &r.RecordedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	err = rows.Err()
	return result, err
}

func (s *Store) DeleteSolutionChangedObjects(ctx context.Context, solutionID uuid.UUID, entityType string, entityXMLIDs []string) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("DeleteSolutionChangedObjects done",
			"solution_id", solutionID,
			"entity_type", entityType,
			"count", len(entityXMLIDs),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("DeleteSolutionChangedObjects start",
		"solution_id", solutionID,
		"entity_type", entityType,
		"count", len(entityXMLIDs),
	)
	if len(entityXMLIDs) == 0 {
		return nil
	}
	_, err = s.pool.Exec(ctx, `
		DELETE FROM solution_changed_objects
		WHERE solution_id=$1 AND entity_type=$2 AND entity_xml_id = ANY($3)`,
		solutionID, entityType, entityXMLIDs)
	return err
}

func (s *Store) MergeWorkingIntoChangedObjects(ctx context.Context, solutionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("MergeWorkingIntoChangedObjects done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("MergeWorkingIntoChangedObjects start", "solution_id", solutionID)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO solution_changed_objects (solution_id, entity_type, entity_xml_id, scheme_xml_id, recorded_at)
		SELECT solution_id, entity_type, entity_xml_id, scheme_xml_id, NOW()
		FROM working_changes
		WHERE solution_id=$1 AND entity_type='graphic_object'
		ON CONFLICT (solution_id, entity_type, entity_xml_id) DO UPDATE SET
			scheme_xml_id = EXCLUDED.scheme_xml_id,
			recorded_at = NOW()`,
		solutionID)
	return err
}

type ExportChangeAckRow struct {
	EntityType  string
	EntityXMLID string
	AckedAt     time.Time
}

func (s *Store) ListExportChangeAcks(ctx context.Context, solutionID uuid.UUID) ([]ExportChangeAckRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT entity_type, entity_xml_id, acked_at
		FROM export_change_acks WHERE solution_id=$1`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ExportChangeAckRow
	for rows.Next() {
		var r ExportChangeAckRow
		if err := rows.Scan(&r.EntityType, &r.EntityXMLID, &r.AckedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) AckExportChangedObjects(ctx context.Context, solutionID uuid.UUID, entityType string, entityXMLIDs []string) error {
	if len(entityXMLIDs) == 0 {
		return nil
	}
	for _, entityXMLID := range entityXMLIDs {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO export_change_acks (solution_id, entity_type, entity_xml_id, acked_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (solution_id, entity_type, entity_xml_id) DO UPDATE SET
				acked_at = NOW()`,
			solutionID, entityType, entityXMLID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteExportChangeAck(ctx context.Context, solutionID uuid.UUID, entityType, entityXMLID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM export_change_acks
		WHERE solution_id=$1 AND entity_type=$2 AND entity_xml_id=$3`,
		solutionID, entityType, entityXMLID)
	return err
}

func (s *Store) ClearExportChangeAcks(ctx context.Context, solutionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM export_change_acks WHERE solution_id=$1`, solutionID)
	return err
}

func (s *Store) DeleteDraftChangesForObjectIDs(ctx context.Context, solutionID uuid.UUID, entityXMLIDs []string) error {
	if len(entityXMLIDs) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM draft_changes
		WHERE entity_xml_id = ANY($1)
		AND session_id IN (SELECT id FROM edit_sessions WHERE solution_id = $2)`,
		entityXMLIDs, solutionID)
	return err
}

func (s *Store) SetSolutionDirty(ctx context.Context, solutionID uuid.UUID, dirty bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE solutions SET is_dirty=$2, updated_at=NOW() WHERE id=$1`, solutionID, dirty)
	return err
}

func (s *Store) ListDisplaySettings(ctx context.Context, userID uuid.UUID) ([]DisplaySetting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT template, renderer, visible_fields, style
		FROM display_settings WHERE user_id=$1 ORDER BY template`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DisplaySetting
	for rows.Next() {
		var d DisplaySetting
		if err := rows.Scan(&d.Template, &d.Renderer, &d.VisibleFields, &d.Style); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) UpsertDisplaySetting(ctx context.Context, userID uuid.UUID, d DisplaySetting) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO display_settings (user_id, template, renderer, visible_fields, style)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id, template) DO UPDATE SET renderer=$3, visible_fields=$4, style=$5`,
		userID, d.Template, d.Renderer, d.VisibleFields, d.Style)
	return err
}

type SolutionTemplateRow struct {
	Template    string `json:"template"`
	ObjectCount int    `json:"object_count"`
}

func (s *Store) UpsertSolutionTemplate(ctx context.Context, solutionID uuid.UUID, template string, count int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO solution_templates (solution_id, template, object_count)
		VALUES ($1, $2, $3)
		ON CONFLICT (solution_id, template) DO UPDATE SET object_count = solution_templates.object_count + EXCLUDED.object_count`,
		solutionID, template, count)
	return err
}

func (s *Store) ListSolutionTemplates(ctx context.Context, solutionID uuid.UUID) ([]SolutionTemplateRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT template, object_count FROM solution_templates
		WHERE solution_id = $1 ORDER BY template`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SolutionTemplateRow
	for rows.Next() {
		var r SolutionTemplateRow
		if err := rows.Scan(&r.Template, &r.ObjectCount); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ListSchemesByProject(ctx context.Context, solutionID uuid.UUID, projectXMLID string) ([]SchemeRow, error) {
	start := time.Now()
	rows, err := s.pool.Query(ctx, `
		SELECT id, solution_id, xml_id, project_xml_id, name, template, payload, payload_blob, payload_encoding
		FROM graphic_schemes WHERE solution_id=$1 AND project_xml_id=$2 ORDER BY name`,
		solutionID, projectXMLID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SchemeRow
	for rows.Next() {
		var id, solID uuid.UUID
		var xmlID, projXMLID, name, template, payloadEncoding string
		var payloadJSONB json.RawMessage
		var payloadBlob []byte
		if scanErr := rows.Scan(&id, &solID, &xmlID, &projXMLID, &name, &template, &payloadJSONB, &payloadBlob, &payloadEncoding); scanErr != nil {
			return nil, scanErr
		}
		row, decodeErr := scanSchemeRow(id, solID, xmlID, projXMLID, name, template, payloadJSONB, payloadBlob, payloadEncoding)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slog.Debug("ListSchemesByProject done",
		"solution_id", solutionID,
		"project_xml_id", projectXMLID,
		"count", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return result, nil
}

func (s *Store) ListSchemeNodesByProject(ctx context.Context, solutionID uuid.UUID, projectXMLID string) ([]NodeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.solution_id, n.parent_id, n.node_type, n.xml_id, n.name, n.tag, n.sort_index, n.meta
		FROM solution_nodes n
		JOIN solution_nodes p ON p.id = n.parent_id
		WHERE n.solution_id = $1 AND p.xml_id = $2 AND n.node_type = 'graphic_scheme'
		ORDER BY n.sort_index`, solutionID, projectXMLID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []NodeRow
	for rows.Next() {
		var r NodeRow
		if err := rows.Scan(&r.ID, &r.SolutionID, &r.ParentID, &r.NodeType, &r.XMLID, &r.Name, &r.Tag, &r.SortIndex, &r.Meta); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
