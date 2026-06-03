package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_StartError(t *testing.T) {
	// No server running → Start() fails → return 1
	code := run([]string{"-server", "127.0.0.1:1", "-local", "0", "-timeout", "100ms"})
	require.Equal(t, 1, code)
}

func TestRun_InvalidFlag(t *testing.T) {
	code := run([]string{"--invalid-flag-value"})
	require.Equal(t, 2, code)
}

func TestRun_WithDirectDomains(t *testing.T) {
	code := run([]string{
		"-local", "0",
		"-server", "127.0.0.1:1",
		"-direct", "example.com",
		"-log", "error",
		"-timeout", "100ms",
	})
	require.Equal(t, 1, code)
}

func TestRun_DefaultDirect(t *testing.T) {
	code := run([]string{
		"-local", "0",
		"-server", "127.0.0.1:1",
		"-default-direct",
		"-log", "error",
		"-timeout", "100ms",
	})
	require.Equal(t, 1, code)
}

func TestRun_LogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "fatal"} {
		t.Run(level, func(t *testing.T) {
			code := run([]string{"-log", level, "-local", "0", "-server", "127.0.0.1:1", "-timeout", "100ms"})
			require.Equal(t, 1, code)
		})
	}
}
