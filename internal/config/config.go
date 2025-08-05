package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/scheduler"
)

type Config struct {
	Username       string
	Password       string
	LancacheServer net.IP
	AdguardAPI     *url.URL
	ServiceNames   []string
	SyncInterval   time.Duration
	Timeout        time.Duration
}

const (
	DefaultTimeout = 30 * time.Second
)

func Load() (*Config, error) {
	config := &Config{
		SyncInterval: scheduler.DefaultSyncInterval,
		Timeout:      DefaultTimeout,
	}

	username := os.Getenv("ADGUARD_USERNAME")
	if username == "" {
		return nil, errors.New("ADGUARD_USERNAME environment variable is required")
	}
	config.Username = username

	password := os.Getenv("ADGUARD_PASSWORD")
	if password == "" {
		return nil, errors.New("ADGUARD_PASSWORD environment variable is required")
	}
	config.Password = password

	lancacheServerStr := os.Getenv("LANCACHE_SERVER")
	if lancacheServerStr == "" {
		return nil, errors.New("LANCACHE_SERVER environment variable is required")
	}
	lancacheServer := net.ParseIP(lancacheServerStr)
	if lancacheServer == nil {
		return nil, fmt.Errorf("invalid LANCACHE_SERVER IP address: %s", lancacheServerStr)
	}
	config.LancacheServer = lancacheServer

	adguardAPIStr := os.Getenv("ADGUARD_API")
	if adguardAPIStr == "" {
		return nil, errors.New("ADGUARD_API environment variable is required")
	}
	adguardAPI, err := url.Parse(adguardAPIStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ADGUARD_API URL: %w", err)
	}
	if adguardAPI.Scheme != "http" && adguardAPI.Scheme != "https" {
		return nil, errors.New("ADGUARD_API must use http or https scheme")
	}
	config.AdguardAPI = adguardAPI

	serviceNamesStr := os.Getenv("SERVICE_NAMES")
	if serviceNamesStr == "" {
		return nil, errors.New("SERVICE_NAMES must be specified (use '*' for all services)")
	}

	var serviceNames []string
	for name := range strings.SplitSeq(serviceNamesStr, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			serviceNames = append(serviceNames, name)
		}
	}

	if len(serviceNames) == 0 {
		return nil, errors.New("SERVICE_NAMES must be specified (use '*' for all services)")
	}
	config.ServiceNames = serviceNames

	if syncIntervalStr := os.Getenv("SYNC_INTERVAL"); syncIntervalStr != "" {
		syncInterval, err := scheduler.ParseSyncInterval(syncIntervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SYNC_INTERVAL: %w", err)
		}
		config.SyncInterval = syncInterval
	}

	return config, nil
}

func (c *Config) IsAllServices() bool {
	return len(c.ServiceNames) == 1 && c.ServiceNames[0] == "*"
}

func (c *Config) HasService(serviceName string) bool {
	if c.IsAllServices() {
		return true
	}
	return slices.Contains(c.ServiceNames, serviceName)
}
