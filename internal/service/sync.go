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

	if err := s.updateFilteringRules(ctx, rewrites); err != nil {
		return fmt.Errorf("failed to update filtering rules: %w", err)
	}

	slog.Info("Filtering rules updated successfully")
	return nil
}

const (
	startMarker = "# lancache-dns-sync start"
	endMarker   = "# lancache-dns-sync end"
)

func (s *SyncService) updateFilteringRules(ctx context.Context, rewrites []types.DNSRewrite) error {
	status, err := s.client.GetFilteringStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get filtering status: %w", err)
	}

	slog.Info("Processing filtering rules", "count", len(rewrites))

	existingRules := status.UserRules
	preservedRules := extractNonManagedRules(existingRules)

	newRules := []string{}
	newRules = append(newRules, preservedRules...)
	newRules = append(newRules, startMarker)

	for _, rewrite := range rewrites {
		rule := fmt.Sprintf("|%s^$dnsrewrite=NOERROR;A;%s,important", rewrite.Domain, rewrite.Answer)
		newRules = append(newRules, rule)
		slog.Debug("Adding DNS rewrite rule", "domain", rewrite.Domain, "answer", rewrite.Answer)
	}

	newRules = append(newRules, endMarker)

	if err := s.client.SetFilteringRules(ctx, newRules); err != nil {
		return fmt.Errorf("failed to set filtering rules: %w", err)
	}

	slog.Info("Successfully updated filtering rules", "total_rules", len(rewrites))
	return nil
}

func extractNonManagedRules(rules []string) []string {
	preserved := []string{}
	inManagedSection := false

	for _, rule := range rules {
		if rule == startMarker {
			inManagedSection = true
			continue
		}
		if rule == endMarker {
			inManagedSection = false
			continue
		}
		if !inManagedSection {
			preserved = append(preserved, rule)
		}
	}

	return preserved
}
