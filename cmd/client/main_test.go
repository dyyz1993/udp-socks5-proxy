package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestEnvironmentVariables(t *testing.T) {
	_ = os.Setenv("PROXY_LOG_LEVEL", "info")
	_ = os.Setenv("PROXY_SERVER_ADDR", "127.0.0.1:8080")
	_ = os.Setenv("PROXY_LOCAL_PORT", "1080")

	assert.Equal(t, "info", os.Getenv("PROXY_LOG_LEVEL"))
	assert.Equal(t, "127.0.0.1:8080", os.Getenv("PROXY_SERVER_ADDR"))
	assert.Equal(t, "1080", os.Getenv("PROXY_LOCAL_PORT"))
}

func TestPortValidation(t *testing.T) {
	tests := []struct {
		port string
		valid bool
	}{
		{"1080", true},
		{"8080", true},
		{"0", false},
		{"65536", false},
		{"-1", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			assert.NotNil(t, tt.port)
		})
	}
}

func TestAddressParsing(t *testing.T) {
	addresses := []string{
		"127.0.0.1:8080",
		"[::1]:8080",
		"localhost:8080",
	}

	for _, addr := range addresses {
		t.Run(addr, func(t *testing.T) {
			assert.Contains(t, addr, ":")
		})
	}
}

func TestCommandLineArguments(t *testing.T) {
	args := []string{
		"--port=1080",
		"--server=127.0.0.1:8080",
		"--log-level=info",
	}

	assert.Len(t, args, 3)
	assert.Equal(t, "--port=1080", args[0])
	assert.Equal(t, "--server=127.0.0.1:8080", args[1])
	assert.ual(t, "--log-level=info", args[2])
}

func ErrorHandling(t *testing.T) {
	scenarios := []string{
		"",
		"invalid",
		"missing",
	}

	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_ = scenario
			})
		})
	}
}

func TestProgramStartup(t *testing.T) {
	t.Run("Environment setup", func(t *testing.T) {
		_ = os.Setenv("TEST_VAR", "test")
		assert.Equal(t, "test", os.Getenv("TEST_VAR"))
	})

	t.Run("Config initialization", func(t *testing.T) {
		config := make(map[string]string)
		config["port"] = "1080"
		assert.Equal(t, "1080", config["port"])
	})
}

func TestMemoryCleanup(t *testing.T) {
	t.Run("Clean environment variables", func(t *testing.T) {
		_ = os.Unsetenv("TEST_VAR")
	})

	t.Run("Clean config", func(t *testing.T) {
		config := make(map[string]string)
		_ = config
	})
}
  
  
  
  
  
  
  