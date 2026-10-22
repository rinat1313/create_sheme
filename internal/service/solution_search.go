package service

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 200
	minSearchQueryLen  = 2
)

type SolutionSearchItem struct {
	ObjectID           string `json:"object_id"`
	TechnologyObjectID string `json:"technology_object_id"`
	Name               string `json:"name"`
	Tag                string `json:"tag"`
	ObjectType         string `json:"object_type"`
	SchemeXMLID        string `json:"scheme_xml_id"`
	SchemeName         string `json:"scheme_name"`
	ProjectXMLID       string `json:"project_xml_id"`
	ProjectName        string `json:"project_name"`
}

type SolutionSearchResult struct {
	Query  string               `json:"query"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
	Items  []SolutionSearchItem `json:"items"`
}

func normalizeSearchPaging(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func prepareSearchQuery(q string) (normalized string, idOnly bool, empty bool) {
	normalized = strings.TrimSpace(q)
	if normalized == "" {
		return "", false, true
	}
	if utf8.RuneCountInString(normalized) < minSearchQueryLen {
		return normalized, true, false
	}
	return normalized, false, false
}

func matchSearchFields(name, tag, objectID, technologyObjectID, template, query string, idOnly bool) bool {
	if query == "" {
		return false
	}
	if idOnly {
		q := strings.ToLower(query)
		return objectID == query ||
			strings.HasPrefix(strings.ToLower(objectID), q) ||
			technologyObjectID == query ||
			strings.HasPrefix(strings.ToLower(technologyObjectID), q)
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(name), q) ||
		strings.Contains(strings.ToLower(tag), q) ||
		strings.Contains(strings.ToLower(objectID), q) ||
		strings.Contains(strings.ToLower(technologyObjectID), q) ||
		strings.Contains(strings.ToLower(template), q)
}

func dedupSearchItems(items []SolutionSearchItem) []SolutionSearchItem {
	seen := map[string]bool{}
	out := make([]SolutionSearchItem, 0, len(items))
	for _, item := range items {
		if item.ObjectID == "" || seen[item.ObjectID] {
			continue
		}
		seen[item.ObjectID] = true
		out = append(out, item)
	}
	return out
}

func applySearchLimit(items []SolutionSearchItem, limit, offset int) []SolutionSearchItem {
	limit, offset = normalizeSearchPaging(limit, offset)
	if offset >= len(items) {
		return []SolutionSearchItem{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func (s *SolutionService) SearchSolution(ctx context.Context, solutionID uuid.UUID, q string, limit, offset int) (result SolutionSearchResult, err error) {
	start := time.Now()
	limit, offset = normalizeSearchPaging(limit, offset)
	result = SolutionSearchResult{
		Query:  strings.TrimSpace(q),
		Limit:  limit,
		Offset: offset,
		Items:  []SolutionSearchItem{},
	}
	defer func() {
		slog.Info("SearchSolution done",
			"solution_id", solutionID,
			"query", result.Query,
			"total", result.Total,
			"limit", result.Limit,
			"offset", result.Offset,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("SearchSolution start", "solution_id", solutionID, "query", q, "limit", limit, "offset", offset)

	normalized, idOnly, empty := prepareSearchQuery(q)
	result.Query = normalized
	if empty {
		return result, nil
	}

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return result, err
	}

	rows, total, searchErr := s.store.SearchSolutionObjects(ctx, solutionID, normalized, idOnly, limit, offset)
	if searchErr != nil {
		err = searchErr
		return result, err
	}
	result.Total = total
	items := make([]SolutionSearchItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, SolutionSearchItem{
			ObjectID:           row.ObjectID,
			TechnologyObjectID: row.TechnologyObjectID,
			Name:               row.Name,
			Tag:                row.Tag,
			ObjectType:         row.ObjectType,
			SchemeXMLID:        row.SchemeXMLID,
			SchemeName:         row.SchemeName,
			ProjectXMLID:       row.ProjectXMLID,
			ProjectName:        row.ProjectName,
		})
	}
	result.Items = dedupSearchItems(items)
	return result, nil
}
