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

type treeFilterNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Type     string
	XMLID    string
}

func NormalizeTreeFilter(mode string, ids []string) (TreeFilter, error) {
	if mode == "" {
		mode = "all"
	}
	switch mode {
	case "all", "schemes":
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

func ExpandTreeFilterSchemeIDs(nodes []treeFilterNode, ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	byID := map[uuid.UUID]treeFilterNode{}
	byXML := map[string]treeFilterNode{}
	children := map[uuid.UUID][]uuid.UUID{}
	for _, n := range nodes {
		byID[n.ID] = n
		if n.XMLID != "" {
			byXML[n.XMLID] = n
		}
		if n.ParentID != nil {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	add := func(xmlID string) {
		if xmlID == "" {
			return
		}
		if _, ok := seen[xmlID]; ok {
			return
		}
		seen[xmlID] = struct{}{}
		out = append(out, xmlID)
	}
	var collectSchemes func(id uuid.UUID)
	collectSchemes = func(id uuid.UUID) {
		n, ok := byID[id]
		if !ok {
			return
		}
		if n.Type == "graphic_scheme" {
			add(n.XMLID)
			return
		}
		for _, child := range children[id] {
			collectSchemes(child)
		}
	}
	for _, id := range ids {
		n, ok := byXML[id]
		if !ok {
			continue
		}
		if n.Type == "graphic_scheme" {
			add(n.XMLID)
			continue
		}
		collectSchemes(n.ID)
	}
	return out
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *SolutionService) expandStoredTreeFilter(ctx context.Context, solutionID uuid.UUID, filter TreeFilter) (TreeFilter, error) {
	if filter.Mode != "schemes" {
		return filter, nil
	}
	nodes, err := s.store.ListNodes(ctx, solutionID)
	if err != nil {
		return filter, err
	}
	refs := make([]treeFilterNode, 0, len(nodes))
	for _, n := range nodes {
		refs = append(refs, treeFilterNode{ID: n.ID, ParentID: n.ParentID, Type: n.NodeType, XMLID: n.XMLID})
	}
	expanded := ExpandTreeFilterSchemeIDs(refs, filter.NodeXMLIDs)
	filter.NodeXMLIDs = expanded
	return filter, nil
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
	mode := row.Mode
	if mode == "projects" {
		mode = "schemes"
	}
	filter, err := NormalizeTreeFilter(mode, row.NodeXMLIDs)
	if err != nil {
		return TreeFilter{}, err
	}
	expanded, err := s.expandStoredTreeFilter(ctx, solutionID, filter)
	if err != nil {
		return TreeFilter{}, err
	}
	if expanded.Mode != row.Mode || !sameStringSlice(expanded.NodeXMLIDs, row.NodeXMLIDs) {
		if upErr := s.store.UpsertTreeFilter(ctx, userID, solutionID, expanded.Mode, expanded.NodeXMLIDs); upErr != nil {
			slog.Warn("GetTreeFilter persist expanded", "user_id", userID, "solution_id", solutionID, "err", upErr)
		}
	}
	return expanded, nil
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
	expanded, err := s.expandStoredTreeFilter(ctx, solutionID, filter)
	if err != nil {
		return TreeFilter{}, err
	}
	if err := s.store.UpsertTreeFilter(ctx, userID, solutionID, expanded.Mode, expanded.NodeXMLIDs); err != nil {
		return TreeFilter{}, err
	}
	return expanded, nil
}
