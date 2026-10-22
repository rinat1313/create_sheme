package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type TagEditRule struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	TextCondition string    `json:"text_condition"`
	Pattern       string    `json:"pattern"`
	Action        string    `json:"action"`
	Replacement   string    `json:"replacement"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
}

type TagEditJob struct {
	ID               uuid.UUID  `json:"id"`
	SolutionID       uuid.UUID  `json:"solution_id"`
	Status           string     `json:"status"`
	TotalObjects     int        `json:"total_objects"`
	ProcessedObjects int        `json:"processed_objects"`
	ChangedObjects   int        `json:"changed_objects"`
	ErrorMessage     string     `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

func (s *Store) ListTagEditRules(ctx context.Context, userID uuid.UUID) ([]TagEditRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, text_condition, pattern, action, replacement, sort_order, created_at
		FROM tag_edit_rules WHERE user_id=$1 ORDER BY sort_order, created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TagEditRule
	for rows.Next() {
		var r TagEditRule
		if err := rows.Scan(&r.ID, &r.UserID, &r.TextCondition, &r.Pattern, &r.Action, &r.Replacement, &r.SortOrder, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) InsertTagEditRule(ctx context.Context, r TagEditRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tag_edit_rules (id, user_id, text_condition, pattern, action, replacement, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		r.ID, r.UserID, r.TextCondition, r.Pattern, r.Action, r.Replacement, r.SortOrder)
	return err
}

func (s *Store) UpdateTagEditRule(ctx context.Context, r TagEditRule) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tag_edit_rules
		SET text_condition=$3, pattern=$4, action=$5, replacement=$6, sort_order=$7
		WHERE id=$1 AND user_id=$2`,
		r.ID, r.UserID, r.TextCondition, r.Pattern, r.Action, r.Replacement, r.SortOrder)
	return err
}

func (s *Store) DeleteTagEditRule(ctx context.Context, userID, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tag_edit_rules WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

func (s *Store) InsertTagEditJob(ctx context.Context, job TagEditJob) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tag_edit_jobs (id, solution_id, status, total_objects, processed_objects, changed_objects, error_message)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		job.ID, job.SolutionID, job.Status, job.TotalObjects, job.ProcessedObjects, job.ChangedObjects, job.ErrorMessage)
	return err
}

func (s *Store) UpdateTagEditJobProgress(ctx context.Context, id uuid.UUID, status string, processed, changed int, errorMessage string, finishedAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tag_edit_jobs
		SET status=$2, processed_objects=$3, changed_objects=$4, error_message=$5, finished_at=$6
		WHERE id=$1`,
		id, status, processed, changed, errorMessage, finishedAt)
	return err
}

func (s *Store) GetTagEditJob(ctx context.Context, id uuid.UUID) (TagEditJob, error) {
	var job TagEditJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, solution_id, status, total_objects, processed_objects, changed_objects, error_message, created_at, finished_at
		FROM tag_edit_jobs WHERE id=$1`, id).Scan(
		&job.ID, &job.SolutionID, &job.Status, &job.TotalObjects, &job.ProcessedObjects,
		&job.ChangedObjects, &job.ErrorMessage, &job.CreatedAt, &job.FinishedAt)
	return job, err
}

func (s *Store) HasActiveTagEditJob(ctx context.Context, solutionID uuid.UUID) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tag_edit_jobs
		WHERE solution_id=$1 AND status IN ('pending', 'running')`, solutionID).Scan(&n)
	return n > 0, err
}
