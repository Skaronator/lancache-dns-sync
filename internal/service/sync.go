package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/skaronator/lancache-dns-sync/internal/client"
	"github.com/skaronator/lancache-dns-sync/internal/config"
	"github.com/skaronator/lancache-dns-sync/internal/domain"
	"github.com/skaronator/lancache-dns-sync/internal/types"
)

type SyncService struct {
	client     client.AdguardClient
	downloader *domain.Downloader
	config     *config.Config
}

func NewSyncService(client client.AdguardClient, downloader *domain.Downloader, cfg *config.Config) *SyncService {
	return &SyncService{
		client:     client,
		downloader: downloader,
		config:     cfg,
	}
}

func (s *SyncService) SyncDomains(ctx context.Context) error {
	slog.Info("Fetching cache domains configuration")
	domains, err := s.downloader.FetchCacheDomains(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch cache domains: %w", err)
	}

	filePaths := s.downloader.GetServiceFilePaths(domains, s.config)
	if len(filePaths) == 0 {
		slog.Info("No domain files to process")
		return nil
	}

	slog.Info("Downloading domains from files", "file_count", len(filePaths))
	rewrites, err := s.downloader.DownloadDomainsFromFiles(ctx, filePaths, s.config.LancacheServer.String())
	if err != nil {
		return fmt.Errorf("failed to download domain files: %w", err)
	}

	slog.Info("Downloaded domain entries", "count", len(rewrites))

	if err := s.updateDNSRewrites(ctx, rewrites); err != nil {
		return fmt.Errorf("failed to update DNS rewrites: %w", err)
	}

	slog.Info("DNS rewrites updated successfully")
	return nil
}

func (s *SyncService) updateDNSRewrites(ctx context.Context, rewrites []types.DNSRewrite) error {
	currentRewrites, err := s.client.GetCurrentRewrites(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current rewrites: %w", err)
	}

	slog.Info("Processing DNS rewrites", "count", len(rewrites))

	for _, rewrite := range rewrites {
		currentAnswer, exists := currentRewrites[rewrite.Domain]

		if exists {
			if currentAnswer != rewrite.Answer {
				slog.Info("Updating DNS rewrite", "domain", rewrite.Domain, "from", currentAnswer, "to", rewrite.Answer)
				if err := s.client.DeleteRewrite(ctx, rewrite.Domain, currentAnswer); err != nil {
					slog.Error("Error deleting rewrite", "domain", rewrite.Domain, "current_answer", currentAnswer, "error", err)
					continue
				}
				if err := s.client.AddRewrite(ctx, rewrite.Domain, rewrite.Answer); err != nil {
					slog.Error("Error adding rewrite", "domain", rewrite.Domain, "error", err)
				}
			} else {
				slog.Info("DNS rewrite already up-to-date", "domain", rewrite.Domain, "answer", rewrite.Answer)
			}
		} else {
			slog.Info("Adding new DNS rewrite", "domain", rewrite.Domain, "answer", rewrite.Answer)
			if err := s.client.AddRewrite(ctx, rewrite.Domain, rewrite.Answer); err != nil {
				slog.Error("Error adding rewrite", "domain", rewrite.Domain, "error", err)
			}
		}
	}

	return nil
}