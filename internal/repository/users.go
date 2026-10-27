package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

var DefaultUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type UserRow struct {
	ID          uuid.UUID
	Login       string
	DisplayName string
	CreatedAt   time.Time
}

func (s *Store) EnsureDefaultUser(ctx context.Context) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("EnsureDefaultUser done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO users (id, login, display_name)
		VALUES ($1, 'default', 'System Default')
		ON CONFLICT (login) DO NOTHING`, DefaultUserID)
	return err
}

func (s *Store) ListUsers(ctx context.Context) ([]UserRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, login, display_name, created_at FROM users ORDER BY login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []UserRow
	for rows.Next() {
		var r UserRow
		if err := rows.Scan(&r.ID, &r.Login, &r.DisplayName, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (UserRow, error) {
	var r UserRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, login, display_name, created_at FROM users WHERE id=$1`, id).Scan(
		&r.ID, &r.Login, &r.DisplayName, &r.CreatedAt)
	return r, err
}

func (s *Store) GetUserByLogin(ctx context.Context, login string) (UserRow, error) {
	var r UserRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, login, display_name, created_at FROM users WHERE login=$1`, login).Scan(
		&r.ID, &r.Login, &r.DisplayName, &r.CreatedAt)
	return r, err
}

func (s *Store) InsertUser(ctx context.Context, r UserRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, login, display_name) VALUES ($1, $2, $3)`,
		r.ID, r.Login, r.DisplayName)
	return err
}

func (s *Store) LinkSolutionToUser(ctx context.Context, userID, solutionID uuid.UUID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_solutions (user_id, solution_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, solution_id) DO NOTHING`,
		userID, solutionID, role)
	return err
}

func (s *Store) UserHasSolution(ctx context.Context, userID, solutionID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_solutions WHERE user_id=$1 AND solution_id=$2)`,
		userID, solutionID).Scan(&exists)
	return exists, err
}

func (s *Store) ListSolutionsForUser(ctx context.Context, userID uuid.UUID) ([]SolutionSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.name, s.company, s.description, s.import_status, s.import_error, s.is_dirty, s.created_at
		FROM solutions s
		JOIN user_solutions us ON us.solution_id = s.id
		WHERE us.user_id = $1
		ORDER BY s.created_at DESC`, userID)
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

func (s *Store) DeleteAllSolutionsForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM solutions
		WHERE id IN (SELECT solution_id FROM user_solutions WHERE user_id=$1)`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *Store) SeedUserSettingsFromDefault(ctx context.Context, newUserID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO display_settings (user_id, template, renderer, visible_fields, style)
		SELECT $1, template, renderer, visible_fields, style
		FROM display_settings WHERE user_id = $2
		ON CONFLICT DO NOTHING`, newUserID, DefaultUserID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO compliance_rules
			(id, user_id, solution_id, template, parameter_scope, entity_template, parameter,
			 value_source, text_condition, expected_text, empty_allowed, indication)
		SELECT gen_random_uuid(), $1, NULL, template, parameter_scope, entity_template, parameter,
		       value_source, text_condition, expected_text, empty_allowed, indication
		FROM compliance_rules
		WHERE user_id = $2 AND solution_id IS NULL`, newUserID, DefaultUserID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO tag_edit_rules (
			id, user_id, text_condition, pattern, action, replacement,
			template, parameter_scope, entity_template, parameter, value_source, actions,
			empty_allowed, check_properties, check_commands, check_protections,
			sort_order)
		SELECT gen_random_uuid(), $1, text_condition, pattern, action, replacement,
		       template, parameter_scope, entity_template, parameter, value_source, actions,
		       empty_allowed, check_properties, check_commands, check_protections,
		       sort_order
		FROM tag_edit_rules WHERE user_id = $2`, newUserID, DefaultUserID)
	return err
}
