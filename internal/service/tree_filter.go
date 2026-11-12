package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInvalidTreeFilterMode = errors.New("invalid tree filter mode")

type TreeFilter struct {
	Mode       string   `json:"mode"`
	NodeXMLIDs []string `json:"node_xml_ids"`
}

func NormalizeTreeFilter(mode string, ids []string) (TreeFilter, error) {
	if mode == "" {
		mode = "all"
	}
	switch mode {
	case "all", "schemes", "projects":
	default:
		return TreeFilter{}, ErrInvalidTreeFilterMode
	}
	if ids == nil {
		ids = []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return TreeFilter{Mode: mode, NodeXMLIDs: out}, nil
}

func (s *SolutionService) GetTreeFilter(ctx context.Context, userID, solutionID uuid.UUID) (TreeFilter, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetTreeFilter done", "user_id", userID, "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("GetTreeFilter start", "user_id", userID, "solution_id", solutionID)

	row, ok, err := s.store.GetTreeFilter(ctx, userID, solutionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TreeFilter{Mode: "all", NodeXMLIDs: []string{}}, nil
		}
		return TreeFilter{}, err
	}
	if !ok {
		return TreeFilter{Mode: "all", NodeXMLIDs: []string{}}, nil
	}
	return NormalizeTreeFilter(row.Mode, row.NodeXMLIDs)
}

func (s *SolutionService) PutTreeFilter(ctx context.Context, userID, solutionID uuid.UUID, mode string, ids []string) (TreeFilter, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("PutTreeFilter done", "user_id", userID, "solution_id", solutionID, "mode", mode, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("PutTreeFilter start", "user_id", userID, "solution_id", solutionID, "mode", mode)

	filter, err := NormalizeTreeFilter(mode, ids)
	if err != nil {
		return TreeFilter{}, err
	}
	if err := s.store.UpsertTreeFilter(ctx, userID, solutionID, filter.Mode, filter.NodeXMLIDs); err != nil {
		return TreeFilter{}, err
	}
	return filter, nil
}
