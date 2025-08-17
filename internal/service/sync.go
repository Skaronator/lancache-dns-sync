package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
	slog.Debug("updateFilteringRules called", "rewrite_count", len(rewrites))

	status, err := s.client.GetFilteringStatus(ctx)
	if err != nil {
		slog.Error("Failed to get filtering status", "error", err)
		return fmt.Errorf("failed to get filtering status: %w", err)
	}
	slog.Debug("Got filtering status", "existing_rules_count", len(status.UserRules))

	slog.Info("Processing filtering rules", "count", len(rewrites))

	existingRules := status.UserRules
	slog.Debug("Existing rules in AdGuard", "count", len(existingRules))

	preservedRules := extractNonManagedRules(existingRules)
	slog.Debug("Preserved non-managed rules", "count", len(preservedRules))

	newRules := []string{}
	newRules = append(newRules, preservedRules...)
	newRules = append(newRules, startMarker)
	slog.Debug("Added start marker", "rules_so_far", len(newRules))

	slog.Debug("Building rewrite rules", "total_rewrites", len(rewrites))
	for _, rewrite := range rewrites {
		var rule string
		if strings.HasPrefix(rewrite.Domain, "*.") {
			// For wildcard domains, use || to match domain and all subdomains
			domain := strings.TrimPrefix(rewrite.Domain, "*.")
			rule = fmt.Sprintf("||%s^$dnsrewrite=%s", domain, rewrite.Answer)
		} else {
			// For exact domains, use | to match only that specific domain
			rule = fmt.Sprintf("|%s^$dnsrewrite=%s", rewrite.Domain, rewrite.Answer)
		}
		newRules = append(newRules, rule)
	}
	slog.Debug("All rewrite rules added", "rules_count_after_rewrites", len(newRules))

	newRules = append(newRules, endMarker)
	slog.Debug("Added end marker", "final_rules_count", len(newRules))

	// Log the actual rules being sent
	slog.Debug("Rules to be sent to AdGuard", "total_count", len(newRules))
	if len(newRules) <= 20 {
		slog.Debug("All rules being sent", "rules", newRules)
	} else {
		slog.Debug("First 10 rules", "rules", newRules[:10])
		slog.Debug("Last 10 rules", "rules", newRules[len(newRules)-10:])
	}

	slog.Debug("Calling SetFilteringRules on AdGuard client")
	if err := s.client.SetFilteringRules(ctx, newRules); err != nil {
		slog.Error("Failed to set filtering rules", "error", err, "rules_count", len(newRules))
		return fmt.Errorf("failed to set filtering rules: %w", err)
	}

	slog.Info("Successfully updated filtering rules", "total_rules", len(rewrites))
	slog.Debug("updateFilteringRules completed", "total_rules_sent", len(newRules))
	return nil
}

func extractNonManagedRules(rules []string) []string {
	slog.Debug("extractNonManagedRules called", "input_rules_count", len(rules))
	preserved := []string{}
	inManagedSection := false
	managedRulesFound := 0

	for i, rule := range rules {
		if rule == startMarker {
			slog.Debug("Found start marker", "at_index", i)
			inManagedSection = true
			continue
		}
		if rule == endMarker {
			slog.Debug("Found end marker", "at_index", i, "managed_rules_found", managedRulesFound)
			inManagedSection = false
			continue
		}
		if !inManagedSection {
			preserved = append(preserved, rule)
		} else {
			managedRulesFound++
			if managedRulesFound <= 3 {
				slog.Debug("Sample managed rule being removed", "rule", rule)
			}
		}
	}

	slog.Debug("extractNonManagedRules completed",
		"preserved_count", len(preserved),
		"managed_removed", managedRulesFound)
	return preserved
}
