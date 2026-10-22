package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisOptions struct {
	URL          string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(ctx context.Context, opts RedisOptions) (Cache, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("NewRedisCache done", "enabled", opts.URL != "", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("NewRedisCache start", "url_set", opts.URL != "")

	if opts.URL == "" {
		return NopCache{}, nil
	}

	opt, parseErr := redis.ParseURL(opts.URL)
	if parseErr != nil {
		err = parseErr
		slog.Warn("redis url parse failed, using NopCache", "err", parseErr)
		return NopCache{}, nil
	}
	if opts.DialTimeout > 0 {
		opt.DialTimeout = opts.DialTimeout
	}
	if opts.ReadTimeout > 0 {
		opt.ReadTimeout = opts.ReadTimeout
	}
	if opts.WriteTimeout > 0 {
		opt.WriteTimeout = opts.WriteTimeout
	}

	client := redis.NewClient(opt)
	pingCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	if opts.DialTimeout <= 0 {
		cancel()
		pingCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
	}
	defer cancel()

	if pingErr := client.Ping(pingCtx).Err(); pingErr != nil {
		_ = client.Close()
		slog.Warn("redis ping failed, using NopCache", "err", pingErr)
		return NopCache{}, nil
	}
	return &RedisCache{client: client}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) (value []byte, hit bool, err error) {
	start := time.Now()
	defer func() {
		slog.Debug("RedisCache.Get done", "hit", hit, "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	val, getErr := r.client.Get(ctx, key).Bytes()
	if getErr == redis.Nil {
		return nil, false, nil
	}
	if getErr != nil {
		err = getErr
		return nil, false, err
	}
	return val, true, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) (err error) {
	start := time.Now()
	defer func() {
		slog.Debug("RedisCache.Set done", "bytes", len(value), "ttl", ttl, "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	err = r.client.Set(ctx, key, value, ttl).Err()
	return err
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) (err error) {
	start := time.Now()
	defer func() {
		slog.Debug("RedisCache.Del done", "keys", len(keys), "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	if len(keys) == 0 {
		return nil
	}
	err = r.client.Del(ctx, keys...).Err()
	return err
}

func (r *RedisCache) Incr(ctx context.Context, key string) (n int64, err error) {
	start := time.Now()
	defer func() {
		slog.Debug("RedisCache.Incr done", "n", n, "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	n, err = r.client.Incr(ctx, key).Result()
	return n, err
}

func (r *RedisCache) Ping(ctx context.Context) (err error) {
	start := time.Now()
	defer func() {
		slog.Debug("RedisCache.Ping done", "err", err, "duration_ms", time.Since(start).Milliseconds())
	}()
	err = r.client.Ping(ctx).Err()
	return err
}

func (r *RedisCache) Enabled() bool { return true }

func (r *RedisCache) Close() (err error) {
	start := time.Now()
	defer func() {
		slog.Info("RedisCache.Close done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	err = r.client.Close()
	return err
}
