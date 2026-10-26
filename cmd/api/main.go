package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"create_sheme/internal/cache"
	"create_sheme/internal/config"
	"create_sheme/internal/handler"
	"create_sheme/internal/logger"
	"create_sheme/internal/repository"
	"create_sheme/internal/service"
)

func main() {
	start := time.Now()
	logger.Init()
	slog.Info("main start")

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://solution:solution@localhost:5432/solution?sslmode=disable"
	}
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}

	ctx := context.Background()
	store, err := repository.New(ctx, dbURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	for _, migrationFile := range []string{"migrations/001_init.sql", "migrations/002_compliance_rules.sql", "migrations/003_display_and_templates.sql", "migrations/004_compliance_value_source.sql", "migrations/005_compliance_parameter_scope.sql", "migrations/006_tag_edit_rules.sql", "migrations/007_tag_edit_jobs.sql", "migrations/008_compliance_duplicate.sql", "migrations/009_solution_metadata.sql", "migrations/010_solution_indexes.sql", "migrations/011_users_and_settings.sql", "migrations/012_import_error.sql", "migrations/013_template_catalog.sql", "migrations/014_export_change_acks.sql", "migrations/015_compliance_duplicate_background.sql", "migrations/016_solution_objects_search.sql", "migrations/017_solution_data_jsonb.sql"} {
		migrationSQL, err := os.ReadFile(migrationFile)
		if err != nil {
			slog.Error("read migration failed", "file", migrationFile, "err", err)
			os.Exit(1)
		}
		if err := store.Migrate(ctx, string(migrationSQL)); err != nil {
			slog.Error("migrate failed", "file", migrationFile, "err", err)
			os.Exit(1)
		}
	}

	if err := store.BackfillSolutionDataFromXML(ctx); err != nil {
		slog.Error("backfill solution data failed", "err", err)
		os.Exit(1)
	}

	migration018, err := os.ReadFile("migrations/018_drop_solution_xml.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/018_drop_solution_xml.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration018)); err != nil {
		slog.Error("migrate failed", "file", "migrations/018_drop_solution_xml.sql", "err", err)
		os.Exit(1)
	}

	migration019, err := os.ReadFile("migrations/019_lean_solution_data.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/019_lean_solution_data.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration019)); err != nil {
		slog.Error("migrate failed", "file", "migrations/019_lean_solution_data.sql", "err", err)
		os.Exit(1)
	}

	if err := store.BackfillLeanSolutionData(ctx); err != nil {
		slog.Error("backfill lean solution data failed", "err", err)
		os.Exit(1)
	}

	migration020, err := os.ReadFile("migrations/020_blob_storage.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/020_blob_storage.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration020)); err != nil {
		slog.Error("migrate failed", "file", "migrations/020_blob_storage.sql", "err", err)
		os.Exit(1)
	}

	migration021, err := os.ReadFile("migrations/021_solution_objects_technology_id.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/021_solution_objects_technology_id.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration021)); err != nil {
		slog.Error("migrate failed", "file", "migrations/021_solution_objects_technology_id.sql", "err", err)
		os.Exit(1)
	}

	migration022, err := os.ReadFile("migrations/022_solution_changed_objects.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/022_solution_changed_objects.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration022)); err != nil {
		slog.Error("migrate failed", "file", "migrations/022_solution_changed_objects.sql", "err", err)
		os.Exit(1)
	}

	migration023, err := os.ReadFile("migrations/023_compliance_duplicate_technology_id.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/023_compliance_duplicate_technology_id.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration023)); err != nil {
		slog.Error("migrate failed", "file", "migrations/023_compliance_duplicate_technology_id.sql", "err", err)
		os.Exit(1)
	}

	migration024, err := os.ReadFile("migrations/024_tag_edit_rule_selection.sql")
	if err != nil {
		slog.Error("read migration failed", "file", "migrations/024_tag_edit_rule_selection.sql", "err", err)
		os.Exit(1)
	}
	if err := store.Migrate(ctx, string(migration024)); err != nil {
		slog.Error("migrate failed", "file", "migrations/024_tag_edit_rule_selection.sql", "err", err)
		os.Exit(1)
	}

	if err := store.EnsureDefaultUser(ctx); err != nil {
		slog.Error("ensure default user failed", "err", err)
		os.Exit(1)
	}

	cfg := config.Load()

	cacheClient, err := cache.NewRedisCache(ctx, cache.RedisOptions{
		URL:          cfg.Cache.RedisURL,
		DialTimeout:  cfg.Cache.DialTimeout,
		ReadTimeout:  cfg.Cache.ReadTimeout,
		WriteTimeout: cfg.Cache.WriteTimeout,
	})
	if err != nil {
		slog.Warn("redis cache init failed, using NopCache", "err", err)
		cacheClient = cache.NopCache{}
	}
	defer func() { _ = cacheClient.Close() }()

	svc := service.NewSolutionServiceWithCache(store, cfg.Import.MaxXMLBytes, cacheClient, cfg.Cache)
	if err := svc.ResumeEnabledBackgroundScanners(ctx); err != nil {
		slog.Error("resume background scanners failed", "err", err)
		os.Exit(1)
	}
	users := service.NewUserService(store)
	api := handler.NewAPI(svc, users, cfg.Import.MaxBytes)

	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/", http.FileServer(http.Dir(webDir)))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           corsMiddleware(handler.UserContextMiddleware(mux)),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		slog.Info("server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("main ready", "duration_ms", time.Since(start).Milliseconds())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	slog.Info("main shutdown")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
