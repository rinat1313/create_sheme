package cache

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) (value []byte, hit bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Incr(ctx context.Context, key string) (int64, error)
	Ping(ctx context.Context) error
	Enabled() bool
	Close() error
}

type NopCache struct{}

func (NopCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	defer func() {
		slog.Debug("NopCache.Get done", "key", key, "duration_ms", time.Since(start).Milliseconds())
	}()
	return nil, false, nil
}

func (NopCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		slog.Debug("NopCache.Set done", "key", key, "ttl", ttl, "bytes", len(value), "duration_ms", time.Since(start).Milliseconds())
	}()
	return nil
}

func (NopCache) Del(ctx context.Context, keys ...string) error {
	start := time.Now()
	defer func() {
		slog.Debug("NopCache.Del done", "keys", len(keys), "duration_ms", time.Since(start).Milliseconds())
	}()
	return nil
}

func (NopCache) Incr(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	defer func() {
		slog.Debug("NopCache.Incr done", "key", key, "duration_ms", time.Since(start).Milliseconds())
	}()
	return 0, nil
}

func (NopCache) Ping(ctx context.Context) error {
	return nil
}

func (NopCache) Enabled() bool { return false }

func (NopCache) Close() error { return nil }

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
	counters map[string]*atomic.Int64
	failGet bool
	failSet bool
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries:  map[string]memoryEntry{},
		counters: map[string]*atomic.Int64{},
	}
}

func (m *MemoryCache) SetFailGet(v bool) { m.failGet = v }
func (m *MemoryCache) SetFailSet(v bool) { m.failSet = v }

func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	var hit bool
	var err error
	defer func() {
		slog.Debug("MemoryCache.Get done", "key", key, "hit", hit, "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	if m.failGet {
		err = errRedisSimulated
		return nil, false, err
	}
	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		m.mu.Lock()
		delete(m.entries, key)
		m.mu.Unlock()
		return nil, false, nil
	}
	hit = true
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, true, nil
}

func (m *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("MemoryCache.Set done", "key", key, "ttl", ttl, "bytes", len(value), "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	if m.failSet {
		err = errRedisSimulated
		return err
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	e := memoryEntry{value: cp}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.entries[key] = e
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Del(ctx context.Context, keys ...string) error {
	start := time.Now()
	defer func() {
		slog.Debug("MemoryCache.Del done", "keys", len(keys), "duration_ms", time.Since(start).Milliseconds())
	}()
	m.mu.Lock()
	for _, k := range keys {
		delete(m.entries, k)
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Incr(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	var n int64
	defer func() {
		slog.Debug("MemoryCache.Incr done", "key", key, "n", n, "duration_ms", time.Since(start).Milliseconds())
	}()
	m.mu.Lock()
	c, ok := m.counters[key]
	if !ok {
		c = &atomic.Int64{}
		m.counters[key] = c
	}
	n = c.Add(1)
	val := []byte(strconv.FormatInt(n, 10))
	m.entries[key] = memoryEntry{value: val}
	m.mu.Unlock()
	return n, nil
}

func (m *MemoryCache) Ping(ctx context.Context) error { return nil }

func (m *MemoryCache) Enabled() bool { return true }

func (m *MemoryCache) Close() error { return nil }

var errRedisSimulated = errString("redis simulated error")

type errString string

func (e errString) Error() string { return string(e) }
