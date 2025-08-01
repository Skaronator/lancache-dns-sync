package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/client"
	"github.com/skaronator/lancache-dns-sync/internal/config"
	"github.com/skaronator/lancache-dns-sync/internal/domain"
	"github.com/skaronator/lancache-dns-sync/internal/service"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		runOnce     = flag.Bool("once", false, "Run once and exit")
		daemon      = flag.Bool("daemon", true, "Run as daemon with scheduling")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("lancache-dns-sync %s, commit %s, built at %s\n", version, commit, date)
		return
	}

	// Check RUN_ONCE environment variable
	runOnceEnv := os.Getenv("RUN_ONCE")
	if runOnceEnv == "true" || runOnceEnv == "1" || runOnceEnv == "yes" {
		*runOnce = true
	}

	slog.Info("Lancache DNS Sync starting", "version", version)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Create services
	adguardClient := client.NewAdguardClient(cfg.AdguardAPI.String(), cfg.Username, cfg.Password, cfg.Timeout)
	downloader := domain.NewDownloader(httpClient)
	syncService := service.NewSyncService(adguardClient, downloader, cfg)

	ctx := context.Background()

	// Run sync once
	if err := syncService.SyncDomains(ctx); err != nil {
		slog.Error("Sync failed", "error", err)
		os.Exit(1)
	}

	// If running once, exit after first sync
	if *runOnce || !*daemon {
		return
	}

	slog.Info("Running in daemon mode", "interval", cfg.SyncInterval)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Info("Running scheduled sync")
			if err := syncService.SyncDomains(ctx); err != nil {
				slog.Error("Scheduled sync failed", "error", err)
			}
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down gracefully", "signal", sig)
			return
		}
	}
}