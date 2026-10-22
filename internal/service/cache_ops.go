package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"create_sheme/internal/cache"
	"create_sheme/internal/config"

	"github.com/google/uuid"
)

func (s *SolutionService) getSolutionGen(ctx context.Context, solutionID uuid.UUID) int64 {
	start := time.Now()
	var n int64
	defer func() {
		slog.Debug("getSolutionGen done", "solution_id", solutionID, "gen", n, "duration_ms", time.Since(start).Milliseconds())
	}()
	val, hit, err := s.cache.Get(ctx, cache.SolutionGenKey(solutionID))
	if err != nil || !hit {
		return 0
	}
	n, _ = strconv.ParseInt(string(val), 10, 64)
	return n
}

func (s *SolutionService) getRulesGen(ctx context.Context, userID uuid.UUID) int64 {
	start := time.Now()
	var n int64
	defer func() {
		slog.Debug("getRulesGen done", "user_id", userID, "gen", n, "duration_ms", time.Since(start).Milliseconds())
	}()
	val, hit, err := s.cache.Get(ctx, cache.RulesGenKey(userID))
	if err != nil || !hit {
		return 0
	}
	n, _ = strconv.ParseInt(string(val), 10, 64)
	return n
}

func (s *SolutionService) BumpSolutionGen(ctx context.Context, solutionID uuid.UUID) {
	start := time.Now()
	var err error
	var n int64
	defer func() {
		slog.Info("BumpSolutionGen done",
			"cache_op", "invalidate",
			"key_kind", "gen",
			"solution_id", solutionID,
			"gen", n,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("BumpSolutionGen start", "solution_id", solutionID)
	n, err = s.cache.Incr(ctx, cache.SolutionGenKey(solutionID))
}

func (s *SolutionService) BumpRulesGen(ctx context.Context, userID uuid.UUID) {
	start := time.Now()
	var err error
	var n int64
	defer func() {
		slog.Info("BumpRulesGen done",
			"cache_op", "invalidate",
			"key_kind", "rulesgen",
			"user_id", userID,
			"gen", n,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("BumpRulesGen start", "user_id", userID)
	n, err = s.cache.Incr(ctx, cache.RulesGenKey(userID))
}

func (s *SolutionService) cacheGet(ctx context.Context, key, keyKind string, solutionID uuid.UUID) ([]byte, bool) {
	start := time.Now()
	val, hit, err := s.cache.Get(ctx, key)
	if err != nil {
		slog.Info("cache get error",
			"cache_op", "miss",
			"key_kind", keyKind,
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
		return nil, false
	}
	op := "miss"
	if hit {
		op = "hit"
	}
	slog.Info("cache get",
		"cache_op", op,
		"key_kind", keyKind,
		"solution_id", solutionID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return val, hit
}

func (s *SolutionService) cacheSet(ctx context.Context, key string, value []byte, ttl time.Duration, keyKind string, solutionID uuid.UUID) {
	start := time.Now()
	if len(value) >= s.cacheCfg.MaxValueBytes {
		slog.Info("cache_skip_too_large",
			"cache_op", "skip",
			"key_kind", keyKind,
			"solution_id", solutionID,
			"size", len(value),
			"max", s.cacheCfg.MaxValueBytes,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}
	if err := s.cache.Set(ctx, key, value, ttl); err != nil {
		slog.Info("cache set error",
			"cache_op", "miss",
			"key_kind", keyKind,
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
		return
	}
	slog.Info("cache set",
		"cache_op", "set",
		"key_kind", keyKind,
		"solution_id", solutionID,
		"bytes", len(value),
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func defaultCacheConfig() config.Cache {
	return config.Cache{
		SchemeTTL:        15 * time.Minute,
		SchemeObjectsTTL: 15 * time.Minute,
		ComplianceTTL:    5 * time.Minute,
		MaxValueBytes:    2097152,
	}
}
