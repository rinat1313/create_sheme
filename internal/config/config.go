package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type HTTP struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type Import struct {
	MaxBytes    int64
	MaxXMLBytes int64
}

type Cache struct {
	RedisURL           string
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	SchemeTTL          time.Duration
	SchemeObjectsTTL   time.Duration
	ComplianceTTL      time.Duration
	MaxValueBytes      int
}

type Config struct {
	HTTP   HTTP
	Import Import
	Cache  Cache
}

func Load() Config {
	cfg := Config{
		HTTP: HTTP{
			ReadHeaderTimeout: durationEnv("HTTP_READ_HEADER_TIMEOUT", 30*time.Second),
			ReadTimeout:       durationEnv("HTTP_READ_TIMEOUT", 30*time.Minute),
			WriteTimeout:      durationEnv("HTTP_WRITE_TIMEOUT", 30*time.Minute),
			IdleTimeout:       durationEnv("HTTP_IDLE_TIMEOUT", 120*time.Second),
		},
		Import: Import{
			MaxBytes: int64Env("IMPORT_MAX_BYTES", 1<<30),
		},
		Cache: Cache{
			RedisURL:         os.Getenv("REDIS_URL"),
			DialTimeout:      durationEnv("REDIS_DIAL_TIMEOUT", 2*time.Second),
			ReadTimeout:      durationEnv("REDIS_READ_TIMEOUT", 500*time.Millisecond),
			WriteTimeout:     durationEnv("REDIS_WRITE_TIMEOUT", 500*time.Millisecond),
			SchemeTTL:        durationEnv("CACHE_SCHEME_TTL", 15*time.Minute),
			SchemeObjectsTTL: durationEnv("CACHE_SCHEME_OBJECTS_TTL", 15*time.Minute),
			ComplianceTTL:    durationEnv("CACHE_COMPLIANCE_TTL", 5*time.Minute),
			MaxValueBytes:    int(int64Env("CACHE_MAX_VALUE_BYTES", 2097152)),
		},
	}
	cfg.Import.MaxXMLBytes = int64Env("IMPORT_MAX_XML_BYTES", cfg.Import.MaxBytes)
	slog.Info("config loaded",
		"http_read_header_timeout", cfg.HTTP.ReadHeaderTimeout,
		"http_read_timeout", cfg.HTTP.ReadTimeout,
		"http_write_timeout", cfg.HTTP.WriteTimeout,
		"http_idle_timeout", cfg.HTTP.IdleTimeout,
		"import_max_bytes", cfg.Import.MaxBytes,
		"import_max_xml_bytes", cfg.Import.MaxXMLBytes,
		"redis_url_set", cfg.Cache.RedisURL != "",
		"cache_scheme_ttl", cfg.Cache.SchemeTTL,
		"cache_scheme_objects_ttl", cfg.Cache.SchemeObjectsTTL,
		"cache_compliance_ttl", cfg.Cache.ComplianceTTL,
		"cache_max_value_bytes", cfg.Cache.MaxValueBytes,
	)
	return cfg
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid duration env, using default", "key", key, "value", raw, "default", fallback)
		return fallback
	}
	return parsed
}

func int64Env(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		slog.Warn("invalid int env, using default", "key", key, "value", raw, "default", fallback)
		return fallback
	}
	if parsed < 1 {
		slog.Warn("int env below minimum, using default", "key", key, "value", parsed, "default", fallback)
		return fallback
	}
	return parsed
}
