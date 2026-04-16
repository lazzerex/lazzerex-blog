package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"lazzerex-blog/internal/api"
	"lazzerex-blog/internal/config"
	"lazzerex-blog/internal/db"
	"lazzerex-blog/internal/logging"
	"lazzerex-blog/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("go service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	dbConn, err := openDatabase(cfg)
	if err != nil {
		return fmt.Errorf("open %s database: %w", cfg.DBDriver, err)
	}
	defer dbConn.Close()

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.ApplyMigrations(startupCtx, dbConn, cfg.DBDriver); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	storeLayer := store.New(dbConn, logger, cfg.DBDriver)
	server := api.NewServer(cfg, logger, storeLayer)

	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("go api server starting", "address", cfg.Address, "database_driver", cfg.DBDriver)
		errCh <- httpServer.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failure: %w", err)
		}
		return nil
	case signalValue := <-sigCh:
		logger.Info("shutdown signal received", "signal", signalValue.String())
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	logger.Info("go api server stopped")
	return nil
}

func openDatabase(cfg config.Config) (*sql.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DBDriver)) {
	case "postgres":
		return openPostgres(cfg.DatabaseURL)
	case "sqlite", "":
		return openSQLite(cfg.DBPath)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
	}
}

func openSQLite(dbPath string) (*sql.DB, error) {
	dbDirectory := filepath.Dir(dbPath)
	if dbDirectory != "." {
		if err := os.MkdirAll(dbDirectory, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	dbConn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := dbConn.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		dbConn.Close()
		return nil, err
	}

	if _, err := dbConn.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		dbConn.Close()
		return nil, err
	}

	return dbConn, nil
}

func openPostgres(databaseURL string) (*sql.DB, error) {
	dbConn, err := sql.Open("postgres", strings.TrimSpace(databaseURL))
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dbConn.PingContext(ctx); err != nil {
		dbConn.Close()
		return nil, err
	}

	return dbConn, nil
}
