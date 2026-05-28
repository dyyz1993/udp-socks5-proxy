package client

import (
	"net"
	"testing"
	"time"

	"github.com/tealife/proxy-cs3/internal/common"
)

func TestNewClient(t *testing.T) {
	config := Config{
		LocalPort:     1080,
		ServerAddr:    "127.0.0.1:1081",
		DirectDomains: []string{"example.com"},
		DefaultDirect: false,
		Timeout:       5 * time.Second,
		LogLevel:      common.InfoLevel,
	}
	cli := NewClient(config, nil)
	if cli == nil {
		t.Fatal("NewClient returned nil")
	}
	if cli.ruleEngine == nil {
		t.Error("ruleEngine should be initialized")
	}
}

func TestNewClient_WithLogger(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	cli := NewClient(Config{LocalPort: 1080}, logger)
	if cli.logger == nil {
		t.Error("logger should be set")
	}
}

func TestClient_StartFail(t *testing.T) {
	config := Config{
		LocalPort:  0,
		ServerAddr: "0.0.0.0:1",
		Timeout:    1 * time.Second,
		LogLevel:   common.DebugLevel,
	}
	cli := NewClient(config, nil)
	err := cli.Start()
	if err == nil {
		cli.Stop()
		t.Log("Start unexpectedly succeeded")
	} else {
		t.Logf("Start failed as expected: %v", err)
	}
}

func TestClient_StopWithoutStart(t *testing.T) {
	cli := NewClient(Config{LocalPort: 1080}, nil)
	if err := cli.Stop(); err != nil {
		t.Errorf("Stop without start should not error: %v", err)
	}
}

func TestClient_StopTwice(t *testing.T) {
	cli := NewClient(Config{LocalPort: 1080}, nil)
	cli.Stop()
	cli.Stop() // should not panic
}

func TestNewGoSocks5Logger(t *testing.T) {
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	gl := NewGoSocks5Logger(logger)
	if gl == nil {
		t.Error("NewGoSocks5Logger returned nil")
	}
}

// parseTargetAddress tests use net.Pipe with proper goroutine setup
func TestParseTargetAddress_Domain(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		// handshake
		client.Write([]byte{0x05, 0x01, 0x00})
		// wait for auth resp
		buf := make([]byte, 10)
		client.Read(buf)
		// domain request: example.com:443
		client.Write([]byte{
			0x05, 0x01, 0x00, 0x03,
			0x0b, 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm',
			0x01, 0xBB,
		})
	}()

	defer server.Close()
	logger := common.NewSimpleLogger("TEST", common.DebugLevel)
	addr, data, err := parseTargetAddress(server, logger)
	if err != nil {
		t.Fatalf("parseTargetAddress: %v", err)
	}
	if addr != "example.com:443" {
		t.Errorf("addr = %q, want example.com:443", addr)
	}
	if len(data) == 0 {
		t.Error("data should not be empty")
	}
}

func TestParseTargetAddress_IPv4(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 10)
		client.Read(buf)
		client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50})
	}()

	defer server.Close()
	addr, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err != nil {
		t.Fatalf("parseTargetAddress: %v", err)
	}
	if addr != "127.0.0.1:80" {
		t.Errorf("addr = %q", addr)
	}
}

func TestParseTargetAddress_IPv6(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 10)
		client.Read(buf)
		// IPv6 ::1 port 80
		client.Write([]byte{
			0x05, 0x01, 0x00, 0x04,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
			0x00, 0x50,
		})
	}()

	defer server.Close()
	addr, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err != nil {
		t.Fatalf("parseTargetAddress: %v", err)
	}
	if addr != "[::1]:80" {
		t.Errorf("addr = %q", addr)
	}
}

func TestParseTargetAddress_InvalidVersion(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x04, 0x01, 0x00}) // invalid version
	}()

	defer server.Close()
	_, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestParseTargetAddress_UnsupportedCmd(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 10)
		client.Read(buf)
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50})
	}()

	defer server.Close()
	_, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err == nil {
		t.Error("expected error for unsupported cmd")
	}
}

func TestParseTargetAddress_UnsupportedAddrType(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 10)
		client.Read(buf)
		client.Write([]byte{0x05, 0x01, 0x00, 0x05})
	}()

	defer server.Close()
	_, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err == nil {
		t.Error("expected error for unsupported addr type")
	}
}

func TestParseTargetAddress_InvalidRsv(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte{0x05, 0x01, 0x00})
		buf := make([]byte, 10)
		client.Read(buf)
		client.Write([]byte{0x05, 0x01, 0x01, 0x01, 0x7f, 0x00, 0x00, 0x01, 0x00, 0x50})
	}()

	defer server.Close()
	_, _, err := parseTargetAddress(server, common.NewSimpleLogger("TEST", common.DebugLevel))
	if err == nil {
		t.Error("expected error for invalid rsv")
	}
}
