package main

import (
	"os"
	"syscall"
	"testing"
	"time"

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
