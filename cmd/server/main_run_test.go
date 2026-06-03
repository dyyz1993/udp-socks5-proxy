package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_StartError(t *testing.T) {
	// Negative port → Start() fails → return 1
	code := run([]string{"-port", "-1", "-log", "error"})
	require.Equal(t, 1, code)
}

func TestRun_InvalidFlag(t *testing.T) {
	code := run([]string{"--invalid-flag-value"})
	require.Equal(t, 2, code)
}

func TestRun_LogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "fatal"} {
		t.Run(level, func(t *testing.T) {
			code := run([]string{"-log", level, "-port", "-1"})
			require.Equal(t, 1, code)
		})
	}
}
