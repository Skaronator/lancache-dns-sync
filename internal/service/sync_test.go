package service

import (
	"testing"
)

func TestExtractNonManagedRules(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "Rules with managed section",
			input: []string{
				"||example.com^",
				"# lancache-dns-sync start",
				"|managed1.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"|managed2.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"# lancache-dns-sync end",
				"||custom.com^",
			},
			expected: []string{
				"||example.com^",
				"||custom.com^",
			},
		},
		{
			name: "No managed section",
			input: []string{
				"||example.com^",
				"||custom.com^",
			},
			expected: []string{
				"||example.com^",
				"||custom.com^",
			},
		},
		{
			name:     "Empty rules",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "Only managed section",
			input: []string{
				"# lancache-dns-sync start",
				"|managed.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"# lancache-dns-sync end",
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNonManagedRules(tt.input)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d rules, got %d", len(tt.expected), len(result))
				return
			}
			
			for i, rule := range result {
				if rule != tt.expected[i] {
					t.Errorf("Expected rule %d to be %s, got %s", i, tt.expected[i], rule)
				}
			}
		})
	}
}

