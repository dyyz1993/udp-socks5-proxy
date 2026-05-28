package common

import (
	"os"
	"os/exec"
	"testing"
)

// TestFatal_ExitCode1 runs as a subprocess to verify Fatal calls os.Exit(1)
func TestFatal_ExitCode1(t *testing.T) {
	if os.Getenv("TEST_FATAL") == "1" {
		logger := NewSimpleLogger("TEST", DebugLevel)
		logger.Fatal("test fatal message")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestFatal_ExitCode1")
	cmd.Env = append(os.Environ(), "TEST_FATAL=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		// Expected: Fatal exits with code 1
		return
	}
	t.Fatalf("Expected exit code 1, got: %v", err)
}

// TestFatalf_ExitCode1 runs as a subprocess to verify Fatalf calls os.Exit(1)
func TestFatalf_ExitCode1(t *testing.T) {
	if os.Getenv("TEST_FATALF") == "1" {
		logger := NewSimpleLogger("TEST", DebugLevel)
		logger.Fatalf("test fatal %s", "message")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestFatalf_ExitCode1")
	cmd.Env = append(os.Environ(), "TEST_FATALF=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return
	}
	t.Fatalf("Expected exit code 1, got: %v", err)
}
