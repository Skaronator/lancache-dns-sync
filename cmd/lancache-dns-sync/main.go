package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	baseURL    = "https://raw.githubusercontent.com/uklans/cache-domains/master/"
	jsonPath   = "cache_domains.json"
	timeoutSec = 30
)

type Config struct {
	Username       string
	Password       string
	LancacheServer string
	AdguardAPI     string
	ServiceNames   []string
	CronSchedule   string
}

type CacheDomain struct {
	Name        string   `json:"name"`
	DomainFiles []string `json:"domain_files"`
}

type CacheDomainsResponse struct {
	CacheDomains []CacheDomain `json:"cache_domains"`
}

type DNSRewrite struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}

type AdguardClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewAdguardClient(baseURL, username, password string) *AdguardClient {
	return &AdguardClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: timeoutSec * time.Second,
		},
	}
}

func (c *AdguardClient) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *AdguardClient) getCurrentRewrites() (map[string]string, error) {
	resp, err := c.makeRequest("GET", "/control/rewrite/list", nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get rewrites: %d", resp.StatusCode)
	}

	var rewrites []DNSRewrite
	if err := json.NewDecoder(resp.Body).Decode(&rewrites); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, rewrite := range rewrites {
		result[rewrite.Domain] = rewrite.Answer
	}

	return result, nil
}

func (c *AdguardClient) addRewrite(domain, answer string) error {
	rewrite := DNSRewrite{Domain: domain, Answer: answer}
	jsonData, err := json.Marshal(rewrite)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "/control/rewrite/add", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add rewrite for %s: %d", domain, resp.StatusCode)
	}

	return nil
}

func (c *AdguardClient) deleteRewrite(domain string) error {
	rewrite := map[string]string{"domain": domain}
	jsonData, err := json.Marshal(rewrite)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "/control/rewrite/delete", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete rewrite for %s: %d", domain, resp.StatusCode)
	}

	return nil
}

func getConfig() (*Config, error) {
	username := os.Getenv("ADGUARD_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("ADGUARD_USERNAME environment variable is required")
	}

	password := os.Getenv("ADGUARD_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("ADGUARD_PASSWORD environment variable is required")
	}

	lancacheServer := os.Getenv("LANCACHE_SERVER")
	if lancacheServer == "" {
		return nil, fmt.Errorf("LANCACHE_SERVER environment variable is required")
	}

	adguardAPI := os.Getenv("ADGUARD_API")
	if adguardAPI == "" {
		return nil, fmt.Errorf("ADGUARD_API environment variable is required")
	}

	serviceNamesStr := os.Getenv("SERVICE_NAMES")
	cronSchedule := os.Getenv("CRON_SCHEDULE")
	if cronSchedule == "" {
		cronSchedule = "0 0 * * *" // Default: daily at midnight
	}

	var serviceNames []string
	if serviceNamesStr != "" {
		for _, name := range strings.Split(serviceNamesStr, ",") {
			serviceNames = append(serviceNames, strings.TrimSpace(name))
		}
	}

	if len(serviceNames) == 0 {
		return nil, fmt.Errorf("SERVICE_NAMES must be specified (use '*' for all services)")
	}

	return &Config{
		Username:       username,
		Password:       password,
		LancacheServer: lancacheServer,
		AdguardAPI:     adguardAPI,
		ServiceNames:   serviceNames,
		CronSchedule:   cronSchedule,
	}, nil
}

func fetchCacheDomains() (*CacheDomainsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec*time.Second)
	defer cancel()

	url := baseURL + jsonPath
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch cache domains: %d", resp.StatusCode)
	}

	var result CacheDomainsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getServiceFilePaths(domains *CacheDomainsResponse, config *Config) []string {
	var filePaths []string

	// Check if user wants all services
	if len(config.ServiceNames) == 1 && config.ServiceNames[0] == "*" {
		for _, domain := range domains.CacheDomains {
			filePaths = append(filePaths, domain.DomainFiles...)
		}
		return filePaths
	}

	// Create map for specific services
	serviceMap := make(map[string]bool)
	for _, name := range config.ServiceNames {
		serviceMap[name] = true
	}

	for _, domain := range domains.CacheDomains {
		if serviceMap[domain.Name] {
			filePaths = append(filePaths, domain.DomainFiles...)
		}
	}

	return filePaths
}

func downloadDomainFile(url string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: %d", url, resp.StatusCode)
	}

	var domains []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			domains = append(domains, line)
		}
	}

	return domains, scanner.Err()
}

func downloadDomainsFromFiles(filePaths []string, lancacheServer string) ([]DNSRewrite, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allRewrites []DNSRewrite

	semaphore := make(chan struct{}, 10) // Limit concurrent downloads

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			url := baseURL + path
			domains, err := downloadDomainFile(url)
			if err != nil {
				slog.Error("Error downloading domain file", "url", url, "error", err)
				return
			}

			var rewrites []DNSRewrite
			for _, domain := range domains {
				rewrites = append(rewrites, DNSRewrite{
					Domain: domain,
					Answer: lancacheServer,
				})
			}

			mu.Lock()
			allRewrites = append(allRewrites, rewrites...)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return allRewrites, nil
}

func updateDNSRewrites(client *AdguardClient, rewrites []DNSRewrite) error {
	currentRewrites, err := client.getCurrentRewrites()
	if err != nil {
		return fmt.Errorf("failed to get current rewrites: %w", err)
	}

	slog.Info("Processing DNS rewrites", "count", len(rewrites))

	for _, rewrite := range rewrites {
		currentAnswer, exists := currentRewrites[rewrite.Domain]

		if exists {
			if currentAnswer != rewrite.Answer {
				slog.Info("Updating DNS rewrite", "domain", rewrite.Domain, "answer", rewrite.Answer)
				if err := client.deleteRewrite(rewrite.Domain); err != nil {
					slog.Error("Error deleting rewrite", "domain", rewrite.Domain, "error", err)
					continue
				}
				if err := client.addRewrite(rewrite.Domain, rewrite.Answer); err != nil {
					slog.Error("Error adding rewrite", "domain", rewrite.Domain, "error", err)
				}
			}
		} else {
			slog.Info("Adding new DNS rewrite", "domain", rewrite.Domain, "answer", rewrite.Answer)
			if err := client.addRewrite(rewrite.Domain, rewrite.Answer); err != nil {
				slog.Error("Error adding rewrite", "domain", rewrite.Domain, "error", err)
			}
		}
	}

	return nil
}

func syncDomains(config *Config) error {
	client := NewAdguardClient(config.AdguardAPI, config.Username, config.Password)

	slog.Info("Fetching cache domains configuration")
	domains, err := fetchCacheDomains()
	if err != nil {
		return fmt.Errorf("failed to fetch cache domains: %w", err)
	}

	filePaths := getServiceFilePaths(domains, config)
	if len(filePaths) == 0 {
		slog.Info("No domain files to process")
		return nil
	}

	slog.Info("Downloading domains from files", "file_count", len(filePaths))
	rewrites, err := downloadDomainsFromFiles(filePaths, config.LancacheServer)
	if err != nil {
		return fmt.Errorf("failed to download domain files: %w", err)
	}

	slog.Info("Downloaded domain entries", "count", len(rewrites))

	if err := updateDNSRewrites(client, rewrites); err != nil {
		return fmt.Errorf("failed to update DNS rewrites: %w", err)
	}

	slog.Info("DNS rewrites updated successfully")
	return nil
}

func parseCronSchedule(cronExpr string) (time.Duration, error) {
	// Try to parse as a simple interval like "1h", "30m", etc. first
	if duration, err := time.ParseDuration(cronExpr); err == nil {
		return duration, nil
	}

	// Parse cron expression (minute hour day month weekday)
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return 24 * time.Hour, fmt.Errorf("invalid cron expression: %s, expected 5 fields", cronExpr)
	}

	minute := fields[0]
	hour := fields[1]
	day := fields[2]
	month := fields[3]
	weekday := fields[4]

	// Handle common patterns
	if minute == "*" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		return time.Minute, nil // Every minute
	}

	if minute != "*" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		return time.Hour, nil // Every hour at specific minute
	}

	if hour != "*" && day == "*" && month == "*" && weekday == "*" {
		return 24 * time.Hour, nil // Daily at specific time
	}

	if weekday != "*" && day == "*" && month == "*" {
		return 7 * 24 * time.Hour, nil // Weekly
	}

	if day != "*" && month == "*" && weekday == "*" {
		return 30 * 24 * time.Hour, nil // Monthly (approximate)
	}

	// For more complex expressions, calculate next run time and use a reasonable interval
	// This is a simplified approach - in production you'd want a proper cron library
	return calculateCronInterval(fields)
}

func calculateCronInterval(fields []string) (time.Duration, error) {
	minute := fields[0]
	hour := fields[1]

	// If we have specific minute and hour, it's likely daily
	if minute != "*" && hour != "*" {
		return 24 * time.Hour, nil
	}

	// If we have specific minute but wildcard hour, it's hourly
	if minute != "*" && hour == "*" {
		return time.Hour, nil
	}

	// If we have specific hour but wildcard minute, treat as daily
	if minute == "*" && hour != "*" {
		return 24 * time.Hour, nil
	}

	// Default to daily for complex expressions
	return 24 * time.Hour, nil
}

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

	config, err := getConfig()
	if err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Run sync once
	if err := syncDomains(config); err != nil {
		slog.Error("Sync failed", "error", err)
		os.Exit(1)
	}

	// If running once, exit after first sync
	if *runOnce || !*daemon {
		return
	}

	// Parse cron schedule for daemon mode
	interval, err := parseCronSchedule(config.CronSchedule)
	if err != nil {
		slog.Warn("Cron schedule parsing warning", "error", err)
	}

	slog.Info("Running in daemon mode", "interval", interval)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Info("Running scheduled sync")
			if err := syncDomains(config); err != nil {
				slog.Error("Scheduled sync failed", "error", err)
			}
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down gracefully", "signal", sig)
			return
		}
	}
}
