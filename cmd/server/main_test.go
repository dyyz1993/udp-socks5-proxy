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
	}
	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
