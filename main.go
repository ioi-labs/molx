package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"nexora-crawl/batch"
	"nexora-crawl/config"
	"nexora-crawl/handlers"
	localMiddleware "nexora-crawl/middleware"
	"nexora-crawl/memstats"
	"nexora-crawl/obscura"
	"nexora-crawl/scraper"
	"nexora-crawl/telemetry"
)

func main() {
	cfg := config.Load()
	initLogger()

	ctx := context.Background()
	shutdownOtel, err := telemetry.Init(ctx, cfg)
	if err != nil {
		slog.Warn("otel init failed", "error", err)
	}
	if shutdownOtel != nil {
		defer shutdownOtel(ctx)
	}

	slog.Info("nexora-crawl starting", "port", cfg.Port, "obscura", cfg.ObscuraBinaryPath, "otel_endpoint", cfg.OTelEndpoint)
	memstats.Log("memory at startup")

	client := obscura.NewClient(cfg.ObscuraBinaryPath, cfg.Timeout)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(telemetry.ChiMiddleware("nexora-crawl"))
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	if len(cfg.AllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           86400,
		}))
	}

	api := localMiddleware.APIKey(cfg)
	v2Scraper := scraper.NewV2(cfg, client)
	fetch := &handlers.FetchHandler{Config: cfg, Client: client}
	scrape := &scraper.V2ScrapeHandler{Config: cfg, Client: client, Scraper: v2Scraper}
	health := &handlers.HealthHandler{Config: cfg}
	spec := &handlers.SpecHandler{}

	batchStore := batch.NewStore()
	batchRunner := batch.NewRunner(batchStore, cfg, client)
	batchCreate := &batch.CreateHandler{Runner: batchRunner}
	batchStatus := &batch.StatusHandler{Store: batchStore}

	v2Search := &handlers.V2SearchHandler{Config: cfg, Scraper: v2Scraper}
	searxSearch := &handlers.SearXNGSearchHandler{Config: cfg}

	r.With(api).Post("/fetch", fetch.ServeHTTP)
	r.With(api).Post("/scrape", scrape.ServeHTTP)
	r.With(api).Post("/v2/scrape", scrape.ServeHTTP)
	r.With(api).Post("/v2/batch/scrape", batchCreate.ServeHTTP)
	r.With(api).Get("/v2/batch/scrape/{id}", batchStatus.ServeHTTP)
	r.With(api).Post("/v2/search", v2Search.ServeHTTP)
	r.With(api).Handle("/search", searxSearch)
	r.Get("/health", health.ServeHTTP)
	r.Get("/reference", handlers.Reference)
	r.Get("/scalar-standalone.js", handlers.ScalarJS)
	r.Get("/openapi.json", spec.ServeHTTP)
	r.Get("/openapi.yaml", spec.ServeHTTP)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	go memstats.LogEvery(60 * time.Second)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCh
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		memstats.Log("memory before request")
		next.ServeHTTP(w, r)
		memstats.Log("memory after request")
		slog.Info("request completed", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr, "elapsed_ms", time.Since(start).Milliseconds())
	})
}

func initLogger() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
}
