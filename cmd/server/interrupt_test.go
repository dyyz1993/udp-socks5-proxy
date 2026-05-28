package main

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
)

func TestWaitForInterrupt_ImmediateSignal(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	config := server.Config{
		Port:     0,
		LogLevel: common.DebugLevel,
	}
	srv := server.NewServer(config, logger)

	go func() {
		time.Sleep(50 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()

	done := make(chan struct{})
	go func() {
		waitForInterrupt(srv, logger)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("waitForInterrupt timed out")
	}
}
