package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
)

func TestWaitForInterrupt_ImmediateSignal(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	config := client.Config{
		LocalPort:  0,
		ServerAddr: "0.0.0.0:1",
		LogLevel:   common.DebugLevel,
	}
	cli := client.NewClient(config, logger)

	// Send SIGINT to self in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()

	// This should complete when signal is received
	done := make(chan struct{})
	go func() {
		waitForInterrupt(cli, logger)
		close(done)
	}()

	select {
	case <-done:
		// Success - waitForInterrupt returned
	case <-time.After(2 * time.Second):
		t.Error("waitForInterrupt timed out")
	}
}

func TestWaitForInterrupt_SIGTERM(t *testing.T) {
	if os.Getenv("TEST_SIGTERM") == "1" {
		logger := common.NewSimpleLogger("test", common.ErrorLevel)
		cli := client.NewClient(client.Config{
			LocalPort:  0,
			ServerAddr: "127.0.0.1:1",
			LogLevel:   common.ErrorLevel,
		}, logger)
		// Don't start - Stop() will error, triggering the error branch
		waitForInterrupt(cli, logger)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestWaitForInterrupt_SIGTERM")
	cmd.Env = append(os.Environ(), "TEST_SIGTERM=1")
	err := cmd.Start()
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}
