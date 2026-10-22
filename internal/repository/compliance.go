package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ComplianceRule struct {
	ID             uuid.UUID       `json:"id"`
	UserID         uuid.UUID       `json:"user_id"`
	SolutionID     *uuid.UUID      `json:"solution_id,omitempty"`
	Template       string          `json:"template"`
	ParameterScope string          `json:"parameter_scope"`
	EntityTemplate string          `json:"entity_template"`
	Parameter      string          `json:"parameter"`
	ValueSource    string          `json:"value_source"`
	TextCondition  string          `json:"text_condition"`
	ExpectedText   string          `json:"expected_text"`
	EmptyAllowed   bool            `json:"empty_allowed"`
	Indication     json.RawMessage `json:"indication"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (s *Store) ListComplianceRules(ctx context.Context, userID uuid.UUID, solutionID *uuid.UUID) ([]ComplianceRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (template, parameter_scope, entity_template, parameter)
		       id, user_id, solution_id, template, parameter_scope, entity_template, parameter,
		       value_source, text_condition, expected_text, empty_allowed, indication, created_at
		FROM compliance_rules
		WHERE user_id = $1 AND (solution_id IS NULL OR ($2::uuid IS NOT NULL AND solution_id = $2))
		ORDER BY template, parameter_scope, entity_template, parameter,
		         solution_id NULLS LAST, created_at DESC`,
		userID, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ComplianceRule
	for rows.Next() {
		var r ComplianceRule
		if err := rows.Scan(&r.ID, &r.UserID, &r.SolutionID, &r.Template, &r.ParameterScope, &r.EntityTemplate,
			&r.Parameter, &r.ValueSource, &r.TextCondition, &r.ExpectedText, &r.EmptyAllowed, &r.Indication, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) InsertComplianceRule(ctx context.Context, r ComplianceRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_rules
			(id, user_id, solution_id, template, parameter_scope, entity_template, parameter,
			 value_source, text_condition, expected_text, empty_allowed, indication)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		r.ID, r.UserID, r.SolutionID, r.Template, r.ParameterScope, r.EntityTemplate, r.Parameter,
		r.ValueSource, r.TextCondition, r.ExpectedText, r.EmptyAllowed, r.Indication)
	return err
}

func (s *Store) UpdateComplianceRule(ctx context.Context, r ComplianceRule) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE compliance_rules
		SET template=$3, parameter_scope=$4, entity_template=$5, parameter=$6, value_source=$7,
		    text_condition=$8, expected_text=$9, empty_allowed=$10, indication=$11
		WHERE id=$1 AND user_id=$2`,
		r.ID, r.UserID, r.Template, r.ParameterScope, r.EntityTemplate, r.Parameter, r.ValueSource,
		r.TextCondition, r.ExpectedText, r.EmptyAllowed, r.Indication)
	return err
}

func (s *Store) DeleteComplianceRule(ctx context.Context, userID, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM compliance_rules WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

func (s *Store) GetComplianceRule(ctx context.Context, userID, id uuid.UUID) (ComplianceRule, error) {
	var r ComplianceRule
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, solution_id, template, parameter_scope, entity_template, parameter,
		       value_source, text_condition, expected_text, empty_allowed, indication, created_at
		FROM compliance_rules WHERE id=$1 AND user_id=$2`, id, userID).Scan(
		&r.ID, &r.UserID, &r.SolutionID, &r.Template, &r.ParameterScope, &r.EntityTemplate,
		&r.Parameter, &r.ValueSource, &r.TextCondition, &r.ExpectedText, &r.EmptyAllowed, &r.Indication, &r.CreatedAt)
	return r, err
}
