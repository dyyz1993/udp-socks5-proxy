package main

import (
	"testing"

	"github.com/tealife/proxy-cs3/internal/common"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  common.LogLevel
	}{
		{"debug", common.DebugLevel},
		{"info", common.InfoLevel},
		{"warn", common.WarnLevel},
		{"error", common.ErrorLevel},
		{"fatal", common.FatalLevel},
		{"unknown", common.InfoLevel},
		{"", common.InfoLevel},
		{"DEBUG", common.InfoLevel}, // case sensitive
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitDomains(t *testing.T) {
	tests := []struct {
		input string
		want  int // expected number of items
	}{
		{"example.com", 1},
		{"", 1},            // current impl appends empty string
		{"a.com,b.com", 1}, // current impl appends whole string as one item
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitDomains(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitDomains(%q) returned %d items, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}
