package cache_test

import (
	"context"
	"testing"
	"time"

	"create_sheme/internal/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestNopCacheMiss(t *testing.T) {
	ctx := context.Background()
	c := cache.NopCache{}
	if c.Enabled() {
		t.Fatal("NopCache should be disabled")
	}
	val, hit, err := c.Get(ctx, "k")
	if err != nil || hit || val != nil {
		t.Fatalf("expected miss, got hit=%v val=%v err=%v", hit, val, err)
	}
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	n, err := c.Incr(ctx, "gen")
	if err != nil || n != 0 {
		t.Fatalf("Nop Incr = %d, %v", n, err)
	}
}

func TestMemoryCacheHitMissInvalidate(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	key := "cs:v1:scheme:x:0:s1"

	_, hit, err := c.Get(ctx, key)
	if err != nil || hit {
		t.Fatalf("expected miss, hit=%v err=%v", hit, err)
	}

	if err := c.Set(ctx, key, []byte(`{"id":"1"}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	val, hit, err := c.Get(ctx, key)
	if err != nil || !hit || string(val) != `{"id":"1"}` {
		t.Fatalf("expected hit, got hit=%v val=%s err=%v", hit, val, err)
	}

	n, err := c.Incr(ctx, "cs:v1:gen:sol")
	if err != nil || n != 1 {
		t.Fatalf("Incr = %d, %v", n, err)
	}
	n, err = c.Incr(ctx, "cs:v1:gen:sol")
	if err != nil || n != 2 {
		t.Fatalf("Incr2 = %d, %v", n, err)
	}

	if err := c.Del(ctx, key); err != nil {
		t.Fatal(err)
	}
	_, hit, err = c.Get(ctx, key)
	if err != nil || hit {
		t.Fatalf("expected miss after del, hit=%v err=%v", hit, err)
	}
}

func TestMemoryCacheErrorFailOpen(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	c.SetFailGet(true)
	_, hit, err := c.Get(ctx, "k")
	if err == nil || hit {
		t.Fatal("expected get error")
	}
	c.SetFailGet(false)
	c.SetFailSet(true)
	if err := c.Set(ctx, "k", []byte("v"), time.Second); err == nil {
		t.Fatal("expected set error")
	}
}

func TestRedisCacheWithMiniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	ctx := context.Background()
	c, err := cache.NewRedisCache(ctx, cache.RedisOptions{
		URL:          "redis://" + mr.Addr() + "/0",
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !c.Enabled() {
		t.Fatal("expected enabled")
	}

	key := "cs:v1:scheme:test:0:sch"
	_, hit, err := c.Get(ctx, key)
	if err != nil || hit {
		t.Fatalf("miss expected, hit=%v err=%v", hit, err)
	}
	if err := c.Set(ctx, key, []byte("payload"), time.Minute); err != nil {
		t.Fatal(err)
	}
	val, hit, err := c.Get(ctx, key)
	if err != nil || !hit || string(val) != "payload" {
		t.Fatalf("hit expected, hit=%v val=%s err=%v", hit, val, err)
	}

	n, err := c.Incr(ctx, "cs:v1:gen:x")
	if err != nil || n != 1 {
		t.Fatalf("incr=%d err=%v", n, err)
	}
}

func TestNewRedisCacheEmptyURL(t *testing.T) {
	c, err := cache.NewRedisCache(context.Background(), cache.RedisOptions{URL: ""})
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() {
		t.Fatal("empty url should yield NopCache")
	}
}

func TestNewRedisCacheBadURLFailOpen(t *testing.T) {
	c, err := cache.NewRedisCache(context.Background(), cache.RedisOptions{
		URL:         "redis://127.0.0.1:1",
		DialTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() {
		t.Fatal("unreachable redis should yield NopCache")
	}
}

func TestKeyBuilders(t *testing.T) {
	sol := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	user := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if got := cache.SchemeKey(sol, 3, "sch1"); got != "cs:v1:scheme:11111111-1111-1111-1111-111111111111:3:sch1" {
		t.Fatalf("SchemeKey=%s", got)
	}
	if got := cache.SchemeObjectsKey(sol, 1, "sch1"); got != "cs:v1:scheme_objects:11111111-1111-1111-1111-111111111111:1:sch1" {
		t.Fatalf("SchemeObjectsKey=%s", got)
	}
	if got := cache.ComplianceKey(user, 2, sol, 4, "all"); got != "cs:v1:compliance:22222222-2222-2222-2222-222222222222:2:11111111-1111-1111-1111-111111111111:4:all" {
		t.Fatalf("ComplianceKey=%s", got)
	}
	if got := cache.ComplianceScope("", ""); got != "all" {
		t.Fatalf("scope all=%s", got)
	}
	if got := cache.ComplianceScope("p1", ""); got != "p:p1" {
		t.Fatalf("scope project=%s", got)
	}
	if got := cache.ComplianceScope("p1", "s1"); got != "s:s1" {
		t.Fatalf("scope scheme=%s", got)
	}
}
