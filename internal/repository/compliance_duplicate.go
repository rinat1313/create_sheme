package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ComplianceDuplicateJob struct {
	ID                   uuid.UUID       `json:"id"`
	SolutionID           uuid.UUID       `json:"solution_id"`
	RuleID               uuid.UUID       `json:"rule_id"`
	Status               string          `json:"status"`
	TotalObjects         int             `json:"total_objects"`
	ProcessedObjects     int             `json:"processed_objects"`
	DuplicateTagCount    int             `json:"duplicate_tag_count"`
	ViolatedObjectCount  int             `json:"violated_object_count"`
	Result               json.RawMessage `json:"result,omitempty"`
	ErrorMessage         string          `json:"error_message"`
	CreatedAt            time.Time       `json:"created_at"`
	FinishedAt           *time.Time      `json:"finished_at,omitempty"`
}

func (s *Store) InsertComplianceDuplicateJob(ctx context.Context, job ComplianceDuplicateJob) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compliance_duplicate_jobs (id, solution_id, rule_id, status, total_objects, processed_objects, duplicate_tag_count, violated_object_count, result, error_message)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		job.ID, job.SolutionID, job.RuleID, job.Status, job.TotalObjects, job.ProcessedObjects, job.DuplicateTagCount, job.ViolatedObjectCount, job.Result, job.ErrorMessage)
	return err
}

func (s *Store) UpdateComplianceDuplicateJobProgress(ctx context.Context, id uuid.UUID, status string, processed, duplicateTagCount, violatedObjectCount int, result json.RawMessage, errorMessage string, finishedAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE compliance_duplicate_jobs
		SET status=$2, processed_objects=$3, duplicate_tag_count=$4, violated_object_count=$5, result=$6, error_message=$7, finished_at=$8
		WHERE id=$1`,
		id, status, processed, duplicateTagCount, violatedObjectCount, result, errorMessage, finishedAt)
	return err
}

func (s *Store) UpdateComplianceDuplicateJobTotal(ctx context.Context, id uuid.UUID, total int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE compliance_duplicate_jobs SET total_objects=$2 WHERE id=$1`, id, total)
	return err
}

func (s *Store) GetComplianceDuplicateJob(ctx context.Context, id uuid.UUID) (ComplianceDuplicateJob, error) {
	var job ComplianceDuplicateJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, solution_id, rule_id, status, total_objects, processed_objects, duplicate_tag_count, violated_object_count, result, error_message, created_at, finished_at
		FROM compliance_duplicate_jobs WHERE id=$1`, id).Scan(
		&job.ID, &job.SolutionID, &job.RuleID, &job.Status, &job.TotalObjects, &job.ProcessedObjects,
		&job.DuplicateTagCount, &job.ViolatedObjectCount, &job.Result, &job.ErrorMessage, &job.CreatedAt, &job.FinishedAt)
	return job, err
}

func (s *Store) GetLatestCompletedComplianceDuplicateJobRuleID(ctx context.Context, solutionID uuid.UUID) (uuid.UUID, error) {
	var ruleID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT rule_id
		FROM compliance_duplicate_jobs
		WHERE solution_id=$1 AND status='completed'
		ORDER BY finished_at DESC NULLS LAST, created_at DESC
		LIMIT 1`, solutionID).Scan(&ruleID)
	return ruleID, err
}
