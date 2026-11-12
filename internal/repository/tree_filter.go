package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TreeFilterRow struct {
	UserID      uuid.UUID
	SolutionID  uuid.UUID
	Mode        string
	NodeXMLIDs  []string
	UpdatedAt   time.Time
}

func (s *Store) GetTreeFilter(ctx context.Context, userID, solutionID uuid.UUID) (TreeFilterRow, bool, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetTreeFilter done", "user_id", userID, "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("GetTreeFilter start", "user_id", userID, "solution_id", solutionID)

	var row TreeFilterRow
	err = s.pool.QueryRow(ctx, `
		SELECT user_id, solution_id, mode, COALESCE(node_xml_ids, '{}'), updated_at
		FROM user_solution_tree_filters
		WHERE user_id=$1 AND solution_id=$2`, userID, solutionID).Scan(
		&row.UserID, &row.SolutionID, &row.Mode, &row.NodeXMLIDs, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TreeFilterRow{}, false, nil
		}
		return TreeFilterRow{}, false, err
	}
	if row.NodeXMLIDs == nil {
		row.NodeXMLIDs = []string{}
	}
	return row, true, nil
}

func (s *Store) UpsertTreeFilter(ctx context.Context, userID, solutionID uuid.UUID, mode string, nodeXMLIDs []string) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("UpsertTreeFilter done", "user_id", userID, "solution_id", solutionID, "mode", mode, "ids", len(nodeXMLIDs), "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("UpsertTreeFilter start", "user_id", userID, "solution_id", solutionID, "mode", mode)

	if nodeXMLIDs == nil {
		nodeXMLIDs = []string{}
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO user_solution_tree_filters (user_id, solution_id, mode, node_xml_ids, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, solution_id) DO UPDATE
		SET mode = EXCLUDED.mode, node_xml_ids = EXCLUDED.node_xml_ids, updated_at = NOW()`,
		userID, solutionID, mode, nodeXMLIDs)
	return err
}
