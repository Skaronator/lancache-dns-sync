package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "all required env vars set",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam,origin",
			},
			wantErr: false,
		},
		{
			name: "missing username",
			envVars: map[string]string{
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing lancache server",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "invalid lancache server IP",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "invalid-ip",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing adguard api",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "invalid adguard api URL",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "invalid-url",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing service names",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
			},
			wantErr: true,
		},
		{
			name: "wildcard service names",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "*",
			},
			wantErr: false,
		},
		{
			name: "custom sync interval",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
				"SYNC_INTERVAL":    "2h",
			},
			wantErr: false,
		},
		{
			name: "invalid sync interval",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
				"SYNC_INTERVAL":    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			os.Clearenv()

			// Set test env vars
			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set env var %s: %v", k, err)
				}
			}

			config, err := Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config != nil {
				if config.SyncInterval <= 0 {
					t.Error("Expected sync interval to be set")
				}
			}
		})
	}
}

func TestConfigIsAllServices(t *testing.T) {
	tests := []struct {
		name         string
		serviceNames []string
		expected     bool
	}{
		{"wildcard", []string{"*"}, true},
		{"specific services", []string{"steam", "origin"}, false},
		{"single service", []string{"steam"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{ServiceNames: tt.serviceNames}
			if got := config.IsAllServices(); got != tt.expected {
				t.Errorf("IsAllServices() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfigHasService(t *testing.T) {
	config := &Config{ServiceNames: []string{"steam", "origin"}}

	tests := []struct {
		serviceName string
		expected    bool
	}{
		{"steam", true},
		{"origin", true},
		{"epic", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName, func(t *testing.T) {
			if got := config.HasService(tt.serviceName); got != tt.expected {
				t.Errorf("HasService(%s) = %v, want %v", tt.serviceName, got, tt.expected)
			}
		})
	}

	// Test wildcard
	wildcardConfig := &Config{ServiceNames: []string{"*"}}
	if !wildcardConfig.HasService("anything") {
		t.Error("Expected wildcard config to match any service")
	}
}