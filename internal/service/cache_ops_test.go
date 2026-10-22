package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"create_sheme/internal/cache"
	"create_sheme/internal/config"

	"github.com/google/uuid"
)

func TestCacheSetSkipsOversized(t *testing.T) {
	mem := cache.NewMemoryCache()
	svc := NewSolutionServiceWithCache(nil, 0, mem, config.Cache{
		SchemeTTL:        time.Minute,
		SchemeObjectsTTL: time.Minute,
		ComplianceTTL:    time.Minute,
		MaxValueBytes:    16,
	})
	solID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	key := cache.SchemeKey(solID, 0, "sch")
	big := make([]byte, 16)
	svc.cacheSet(context.Background(), key, big, time.Minute, "scheme", solID)
	_, hit, err := mem.Get(context.Background(), key)
	if err != nil || hit {
		t.Fatalf("oversized value must not be stored, hit=%v err=%v", hit, err)
	}

	ok := []byte(`{"ok":true}`)
	svc.cacheSet(context.Background(), key, ok, time.Minute, "scheme", solID)
	val, hit, err := mem.Get(context.Background(), key)
	if err != nil || !hit || string(val) != string(ok) {
		t.Fatalf("expected stored value, hit=%v val=%s err=%v", hit, val, err)
	}
}

func TestCacheGetRedisErrorIsMiss(t *testing.T) {
	mem := cache.NewMemoryCache()
	mem.SetFailGet(true)
	svc := NewSolutionServiceWithCache(nil, 0, mem, defaultCacheConfig())
	solID := uuid.New()
	key := cache.SchemeKey(solID, 0, "sch")
	_, hit := svc.cacheGet(context.Background(), key, "scheme", solID)
	if hit {
		t.Fatal("redis error must fail-open as miss")
	}
}

func TestBumpSolutionGenInvalidatesKeySpace(t *testing.T) {
	mem := cache.NewMemoryCache()
	svc := NewSolutionServiceWithCache(nil, 0, mem, defaultCacheConfig())
	solID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	ctx := context.Background()

	key0 := cache.SchemeKey(solID, 0, "sch")
	if err := mem.Set(ctx, key0, []byte(`{"v":0}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	svc.BumpSolutionGen(ctx, solID)
	gen := svc.getSolutionGen(ctx, solID)
	if gen != 1 {
		t.Fatalf("gen=%d want 1", gen)
	}
	key1 := cache.SchemeKey(solID, gen, "sch")
	if key0 == key1 {
		t.Fatal("gen bump must change scheme key")
	}
	_, hit, _ := mem.Get(ctx, key1)
	if hit {
		t.Fatal("new gen key must miss until repopulated")
	}
}

func TestBumpRulesGenChangesComplianceKey(t *testing.T) {
	mem := cache.NewMemoryCache()
	svc := NewSolutionServiceWithCache(nil, 0, mem, defaultCacheConfig())
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	solID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	ctx := context.Background()

	before := cache.ComplianceKey(userID, svc.getRulesGen(ctx, userID), solID, 0, "all")
	svc.BumpRulesGen(ctx, userID)
	after := cache.ComplianceKey(userID, svc.getRulesGen(ctx, userID), solID, 0, "all")
	if before == after {
		t.Fatal("rules gen bump must change compliance key")
	}
}

func TestCacheSetFailOpenOnSetError(t *testing.T) {
	mem := cache.NewMemoryCache()
	mem.SetFailSet(true)
	svc := NewSolutionServiceWithCache(nil, 0, mem, defaultCacheConfig())
	solID := uuid.New()
	key := cache.SchemeKey(solID, 0, "sch")
	svc.cacheSet(context.Background(), key, []byte(`{}`), time.Minute, "scheme", solID)
	_, hit, _ := mem.Get(context.Background(), key)
	if hit {
		t.Fatal("failed set must not leave value")
	}
}

func TestComplianceScopeHelpers(t *testing.T) {
	report := ComplianceReport{Nodes: []ComplianceNodeHint{{XMLID: "a", Type: "project", Name: "P"}}}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	mem := cache.NewMemoryCache()
	svc := NewSolutionServiceWithCache(nil, 0, mem, defaultCacheConfig())
	solID := uuid.New()
	userID := uuid.New()
	key := cache.ComplianceKey(userID, 0, solID, 0, "all")
	svc.cacheSet(context.Background(), key, raw, time.Minute, "compliance", solID)
	got, hit := svc.cacheGet(context.Background(), key, "compliance", solID)
	if !hit {
		t.Fatal("expected hit")
	}
	var decoded ComplianceReport
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Nodes) != 1 || decoded.Nodes[0].XMLID != "a" {
		t.Fatalf("decoded=%+v", decoded)
	}
}
