package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/httpapi"
	"github.com/UnitVectorY-Labs/remventory/internal/mcpserver"
	"github.com/UnitVectorY-Labs/remventory/internal/remy"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	mcptransport "github.com/mark3labs/mcp-go/server"
)

// Version is the application version, injected at build time via ldflags.
var Version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("remventory stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	resolveVersion()

	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repo *store.Store
	if cfg.DatabaseURL != "" {
		db, err := store.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		if cfg.AutoMigrate {
			if err := db.Migrate(ctx); err != nil {
				return fmt.Errorf("run migrations: %w", err)
			}
		}

		if err := db.EnsureDefaultUser(ctx, cfg.DefaultUserName); err != nil {
			return fmt.Errorf("ensure default user: %w", err)
		}

		repo = db
	} else {
		logger.Warn("DATABASE_URL is not set; database-backed routes will report unavailable")
	}

	remyService := remy.New(cfg, repo)
	var mcpHandler http.Handler
	if repo != nil {
		mcpHandler = mcptransport.NewStreamableHTTPServer(mcpserver.New(Version, repo, remyService))
	}

	api := httpapi.New(httpapi.Options{
		Config:     cfg,
		Store:      repo,
		Remy:       remyService,
		MCPHandler: mcpHandler,
		Version:    Version,
		Logger:     logger,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("starting remventory", "version", Version, "addr", cfg.HTTPAddr)
		errc <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func resolveVersion() {
	if Version != "dev" && Version != "" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			Version = bi.Main.Version
		}
	}
}
