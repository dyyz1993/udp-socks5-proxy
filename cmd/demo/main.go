package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tealife/proxy-cs3/internal/client"
	"github.com/tealife/proxy-cs3/internal/common"
	"github.com/tealife/proxy-cs3/internal/server"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// 颜色常量
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// 步骤计数器
var stepNum int
var stepMu sync.Mutex

func nextStep(title string) int {
	stepMu.Lock()
	defer stepMu.Unlock()
	stepNum++
	fmt.Printf("\n%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s步骤 %d: %s%s\n", colorBold+colorGreen, stepNum, title, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorCyan, colorReset)
	return stepNum
}

func printBanner() {
	fmt.Printf("\n%s", colorBold+colorCyan)
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          UDP-SOCKS5 Proxy 通信流程可视化演示                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("%s\n", colorReset)

	fmt.Printf("%s架构图:%s\n\n", colorBold, colorReset)
	fmt.Printf("  %s┌──────────┐%s   TCP    %s┌──────────────┐%s   UDP   %s┌──────────────┐%s   TCP   %s┌────────┐%s\n",
		colorGreen, colorReset, colorYellow, colorReset, colorPurple, colorReset, colorBlue, colorReset)
	fmt.Printf("  %s│  curl /  │%s ◄─────► %s│  SOCKS5      │%s ◄─────► %s│  SOCKS5      │%s ◄─────► %s│ Echo   │%s\n",
		colorGreen, colorReset, colorYellow, colorReset, colorPurple, colorReset, colorBlue, colorReset)
	fmt.Printf("  %s│  browser │%s         %s│  Client:1080 │%s         %s│  Server:1081 │%s         %s│ Server │%s\n",
		colorGreen, colorReset, colorYellow, colorReset, colorPurple, colorReset, colorBlue, colorReset)
	fmt.Printf("  %s└──────────┘%s         %s└──────────────┘%s         %s└──────────────┘%s         %s└────────┘%s\n\n",
		colorGreen, colorReset, colorYellow, colorReset, colorPurple, colorReset, colorBlue, colorReset)

	fmt.Printf("%s数据包类型:%s\n", colorBold, colorReset)
	fmt.Printf("  %s1-握手%s  %s2-数据%s  %s3-心跳%s  %s4-关闭%s  %s5-错误%s  %s6-分片%s\n\n",
		colorYellow, colorReset, colorGreen, colorReset, colorPurple, colorReset, colorRed, colorReset, colorRed, colorReset, colorCyan, colorReset)
}

func printPacketDetail(label string, data []byte, direction string) {
	if len(data) == 0 {
		fmt.Printf("  %s%s: (空)%s\n", colorDim, label, colorReset)
		return
	}

	// 方向箭头
	arrow := "→"
	if strings.Contains(direction, "←") || strings.Contains(direction, "recv") || strings.Contains(direction, "from") {
		arrow = "←"
	}

	// 尝试解析为 tunnel packet
	parsed := false
	if len(data) >= 5 {
		pkt, err := tunnel.ParsePacket(data)
		if err == nil {
			parsed = true
			typeName := packetTypeName(pkt.Header.Type)
			typeColor := packetTypeColor(pkt.Header.Type)

			fmt.Printf("  %s%s %s[%s]%s (长度: %d 字节)\n", arrow, label, typeColor, typeName, colorReset, len(data))
			fmt.Printf("    %s版本:%s %d  %s类型:%s %s%d%s  %s标志:%s %d\n",
				colorDim, colorReset, pkt.Header.Version,
				colorDim, colorReset, typeColor, pkt.Header.Type, colorReset,
				colorDim, colorReset, pkt.Header.Flags)
			fmt.Printf("    %s连接ID:%s %.12s...  %s流ID:%s %.12s...\n",
				colorDim, colorReset, pkt.Header.ConnectionID,
				colorDim, colorReset, pkt.Header.StreamID)

			// 显示数据摘要
			if len(pkt.Data) > 0 {
				showLen := len(pkt.Data)
				if showLen > 48 {
					showLen = 48
				}
				fmt.Printf("    %s数据(%d字节):%s %x", colorDim, len(pkt.Data), colorReset, pkt.Data[:showLen])
				if len(pkt.Data) > 48 {
					fmt.Printf("%s...%s", colorDim, colorReset)
				}
				fmt.Println()

				// SOCKS5 协议分析
				analyzeSOCKS5(pkt.Data)
			}
		}
	}

	if !parsed {
		fmt.Printf("  %s%s: %s(原始数据, %d字节)%s\n", arrow, label, colorDim, len(data), colorReset)
		showLen := len(data)
		if showLen > 48 {
			showLen = 48
		}
		fmt.Printf("    %x", data[:showLen])
		if len(data) > 48 {
			fmt.Printf("%s...%s", colorDim, colorReset)
		}
		fmt.Println()
	}
}

func packetTypeName(t tunnel.PacketType) string {
	switch t {
	case tunnel.PacketTypeHandshake:
		return "握手"
	case tunnel.PacketTypeData:
		return "数据"
	case tunnel.PacketTypeHeartbeat:
		return "心跳"
	case tunnel.PacketTypeClose:
		return "关闭"
	case tunnel.PacketTypeError:
		return "错误"
	case tunnel.PacketTypeFragmented:
		return "分片"
	default:
		return fmt.Sprintf("未知(%d)", t)
	}
}

func packetTypeColor(t tunnel.PacketType) string {
	switch t {
	case tunnel.PacketTypeHandshake:
		return colorYellow
	case tunnel.PacketTypeData:
		return colorGreen
	case tunnel.PacketTypeHeartbeat:
		return colorPurple
	case tunnel.PacketTypeClose:
		return colorRed
	case tunnel.PacketTypeError:
		return colorRed
	case tunnel.PacketTypeFragmented:
		return colorCyan
	default:
		return colorWhite
	}
}

func analyzeSOCKS5(data []byte) {
	if len(data) < 2 || data[0] != 0x05 {
		return
	}

	fmt.Printf("    %s── SOCKS5 分析 ──%s\n", colorCyan, colorReset)

	if len(data) == 2 && data[1] == 0x00 {
		fmt.Printf("    %s  ✓ 认证响应: NO AUTH%s\n", colorGreen, colorReset)
		return
	}

	if len(data) >= 3 && data[1] == 0x01 {
		fmt.Printf("    %s  → CONNECT 请求: 版本=5, 方法数=%d%s\n", colorYellow, data[1], colorReset)
	}

	if len(data) >= 4 && data[1] == 0x00 && data[2] == 0x00 {
		switch data[3] {
		case 0x01:
			if len(data) >= 10 {
				ip := net.IP(data[4:8])
				port := int(data[8])<<8 | int(data[9])
				fmt.Printf("    %s  ✓ CONNECT 响应: IPv4 %s:%d%s\n", colorGreen, ip, port, colorReset)
			}
		case 0x03:
			if len(data) >= 5 {
				domainLen := int(data[4])
				if len(data) >= 5+domainLen+2 {
					domain := string(data[5 : 5+domainLen])
					port := int(data[5+domainLen])<<8 | int(data[5+domainLen+1])
					fmt.Printf("    %s  → 目标域名: %s:%d%s\n", colorYellow, domain, port, colorReset)
				}
			}
		case 0x04:
			if len(data) >= 22 {
				ip := net.IP(data[4:20])
				port := int(data[20])<<8 | int(data[21])
				fmt.Printf("    %s  → 目标 IPv6: [%s]:%d%s\n", colorYellow, ip, port, colorReset)
			}
		}
	}

	// 检测 HTTP 数据
	if len(data) > 5 {
		if bytes.HasPrefix(data, []byte("HTTP/")) || bytes.HasPrefix(data, []byte("GET ")) ||
			bytes.HasPrefix(data, []byte("POST ")) || bytes.HasPrefix(data, []byte("CONNECT ")) {
			lines := strings.Split(string(data), "\r\n")
			showLines := len(lines)
			if showLines > 3 {
				showLines = 3
			}
			fmt.Printf("    %s  → HTTP: %s%s\n", colorYellow, lines[0], colorReset)
			for i := 1; i < showLines; i++ {
				fmt.Printf("    %s    %s%s\n", colorDim, lines[i], colorReset)
			}
		}
	}
}

// ============================================================
// 拦截式日志记录器 — 捕获所有通信事件
// ============================================================

type demoLogger struct {
	prefix string
	level  common.LogLevel
}

func newDemoLogger(prefix string) *demoLogger {
	return &demoLogger{prefix: prefix, level: common.DebugLevel}
}

func (l *demoLogger) log(level common.LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf(format, args...)

	prefixColor := colorYellow
	if l.prefix == "SERVER" {
		prefixColor = colorPurple
	}

	// 检测关键事件并高亮
	highlight := ""
	if strings.Contains(msg, "握手") || strings.Contains(msg, "Handshake") {
		highlight = colorYellow
	} else if strings.Contains(msg, "隧道流") || strings.Contains(msg, "stream") || strings.Contains(msg, "StreamID") {
		highlight = colorGreen
	} else if strings.Contains(msg, "心跳") || strings.Contains(msg, "Heartbeat") {
		highlight = colorPurple
	} else if strings.Contains(msg, "十六进制") {
		// 提取 hex 数据并格式化
		parts := strings.SplitN(msg, "十六进制数据: ", 2)
		if len(parts) == 2 {
			fmt.Printf("  %s%s[%s]%s 📦 原始数据: %s%s\n",
				colorDim, time.Now().Format("15:04:05.000"), l.prefix, colorReset,
				prefixColor, parts[1])
		}
		return
	} else if strings.Contains(msg, "创建") || strings.Contains(msg, "已启动") {
		highlight = colorGreen
	}

	_ = highlight
	fmt.Printf("  %s%s[%s]%s %s\n",
		colorDim, time.Now().Format("15:04:05.000"), l.prefix, colorReset, msg)
}

func (l *demoLogger) Debug(args ...interface{}) {
	l.log(common.DebugLevel, "%v", args...)
}
func (l *demoLogger) Debugf(format string, args ...interface{}) {
	l.log(common.DebugLevel, format, args...)
}
func (l *demoLogger) Info(args ...interface{}) {
	l.log(common.InfoLevel, "%v", args...)
}
func (l *demoLogger) Infof(format string, args ...interface{}) {
	l.log(common.InfoLevel, format, args...)
}
func (l *demoLogger) Warn(args ...interface{}) {
	l.log(common.WarnLevel, "%v", args...)
}
func (l *demoLogger) Warnf(format string, args ...interface{}) {
	l.log(common.WarnLevel, format, args...)
}
func (l *demoLogger) Error(args ...interface{}) {
	l.log(common.ErrorLevel, "%v", args...)
}
func (l *demoLogger) Errorf(format string, args ...interface{}) {
	l.log(common.ErrorLevel, format, args...)
}
func (l *demoLogger) Fatal(args ...interface{}) {
	l.log(common.FatalLevel, "%v", args...)
}
func (l *demoLogger) Fatalf(format string, args ...interface{}) {
	l.log(common.FatalLevel, format, args...)
}

// ============================================================
// Echo 目标服务器
// ============================================================

func startEchoServer() (int, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("启动 Echo 服务器失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					// 解析 HTTP 请求并返回响应
					req := string(buf[:n])
					if strings.HasPrefix(req, "GET ") || strings.HasPrefix(req, "POST ") {
						body := fmt.Sprintf("Hello from Echo Server! You requested: %s\nTime: %s\nProxy chain: curl → SOCKS5 Client → UDP Tunnel → SOCKS5 Server → Echo Server",
							strings.Split(req, " ")[1], time.Now().Format("2006-01-02 15:04:05"))
						resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
						c.Write([]byte(resp))
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	return port, func() { ln.Close() }
}

// ============================================================
// UDP 包嗅探器 — 拦截本地 UDP 通信
// ============================================================

type udpSniffer struct {
	clientAddr *net.UDPAddr
	serverAddr *net.UDPAddr
	clientConn *net.UDPConn
	serverConn *net.UDPConn
	done       chan struct{}
}

// startSniffer 启动一个 UDP 中继嗅探器
// 它监听一个中间端口，转发 client ↔ server 之间的数据
func startSniffer(realServerPort int) (int, *udpSniffer) {
	// 嗅探器监听端口
	snifferAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	snifferConn, err := net.ListenUDP("udp", snifferAddr)
	if err != nil {
		log.Fatalf("启动嗅探器失败: %v", err)
	}
	snifferPort := snifferConn.LocalAddr().(*net.UDPAddr).Port

	// 连接到真实服务器
	realAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", realServerPort))

	s := &udpSniffer{
		serverAddr: realAddr,
		clientConn: snifferConn,
		done:       make(chan struct{}),
	}

	// 启动转发
	go s.relay()

	return snifferPort, s
}

func (s *udpSniffer) relay() {
	buf := make([]byte, 65536)
	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := s.clientConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		// 判断方向
		if s.clientAddr == nil {
			s.clientAddr = addr
		}

		fmt.Printf("\n  %s── UDP 数据包 ──%s\n", colorCyan, colorReset)
		printPacketDetail(fmt.Sprintf("客户端 %s → 服务器", addr), data, "→")

		// 转发到真实服务器
		s.clientConn.WriteToUDP(data, s.serverAddr)

		// 读取服务器响应
		s.clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err = s.clientConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		respData := make([]byte, n)
		copy(respData, buf[:n])

		fmt.Printf("\n  %s── UDP 响应包 ──%s\n", colorCyan, colorReset)
		printPacketDetail("服务器 → 客户端", respData, "←")

		// 转发回客户端
		s.clientConn.WriteToUDP(respData, addr)
	}
}

func (s *udpSniffer) stop() {
	close(s.done)
	s.clientConn.Close()
}

// ============================================================
// SOCKS5 握手演示（纯协议分析）
// ============================================================

func demoSOCKS5Protocol(targetAddr string) {
	nextStep("SOCKS5 协议握手详解 (直接协议分析)")

	fmt.Printf("  %s目标地址: %s%s\n", colorYellow, targetAddr, colorReset)
	fmt.Println()

	// 步骤 1: 客户端发送握手请求
	fmt.Printf("  %s1. 客户端 → 代理: 版本协商%s\n", colorBold+colorYellow, colorReset)
	fmt.Printf("     %s发送:%s [05 01 00]\n", colorGreen, colorReset)
	fmt.Printf("     %s含义:%s VER=5(协议版本) NMETHODS=1(1种认证) METHODS=[00(无认证)]\n", colorDim, colorReset)

	// 步骤 2: 代理回应
	fmt.Printf("\n  %s2. 代理 → 客户端: 选择认证方式%s\n", colorBold+colorYellow, colorReset)
	fmt.Printf("     %s发送:%s [05 00]\n", colorGreen, colorReset)
	fmt.Printf("     %s含义:%s VER=5 METHOD=00(选择无认证)\n", colorDim, colorReset)

	// 步骤 3: 客户端发送连接请求
	fmt.Printf("\n  %s3. 客户端 → 代理: CONNECT 请求%s\n", colorBold+colorYellow, colorReset)
	host, portStr, _ := net.SplitHostPort(targetAddr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	fmt.Printf("     %s发送:%s [05 01 00 03 %02x", colorGreen, colorReset, len(host))
	for _, b := range []byte(host) {
		fmt.Printf(" %02x", b)
	}
	fmt.Printf(" %02x %02x]\n", port>>8, port&0xff)
	fmt.Printf("     %s含义:%s VER=5 CMD=01(CONNECT) RSV=00 ATYP=03(域名)\n", colorDim, colorReset)
	fmt.Printf("     %s      域名长度=%d 域名=%s 端口=%d%s\n", colorDim, len(host), host, port, colorReset)

	// 步骤 4: 代理回应连接成功
	fmt.Printf("\n  %s4. 代理 → 客户端: 连接成功响应%s\n", colorBold+colorYellow, colorReset)
	fmt.Printf("     %s发送:%s [05 00 00 01 00 00 00 00 00 00]\n", colorGreen, colorReset)
	fmt.Printf("     %s含义:%s VER=5 REP=00(成功) RSV=00 ATYP=01(IPv4)\n", colorDim, colorReset)
	fmt.Printf("     %s      BND.ADDR=0.0.0.0 BND.PORT=0%s\n", colorDim, colorReset)
}

// ============================================================
// Tunnel 协议演示
// ============================================================

func demoTunnelProtocol() {
	nextStep("UDP Tunnel 隧道协议详解")

	fmt.Printf("  %sTunnel 数据包格式:%s\n\n", colorBold, colorReset)
	fmt.Printf("  %s┌──────────────────────────────────────────────┐%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│ Version (1B) │ Type (1B) │ Flags (1B)       │%s\n", colorCyan, colorReset)
	fmt.Printf("  %s├──────────────────────────────────────────────┤%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│ ConnectionID Length (2B) │ ConnectionID     │%s\n", colorCyan, colorReset)
	fmt.Printf("  %s├──────────────────────────────────────────────┤%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│ StreamID Length (2B) │ StreamID (Data only) │%s\n", colorCyan, colorReset)
	fmt.Printf("  %s├──────────────────────────────────────────────┤%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│ Payload (变长)                                │%s\n", colorCyan, colorReset)
	fmt.Printf("  %s└──────────────────────────────────────────────┘%s\n\n", colorCyan, colorReset)

	// 演示握手包
	fmt.Printf("  %s握手包 (Type=1):%s\n", colorBold+colorYellow, colorReset)
	key := [32]byte{}
	copy(key[:], "demo-secret-key-for-visualization")
	pkt := tunnel.NewHandshakePacket("temp-001", key, "default", 0, "demo-1.0")
	fmt.Printf("    %s包大小:%s %d 字节\n", colorDim, colorReset, len(pkt.Bytes()))
	fmt.Printf("    %s包含:%s 密钥(32B) + 分组名 + 特性标志 + 版本号\n", colorDim, colorReset)

	// 演示数据包
	fmt.Printf("\n  %s数据包 (Type=2):%s\n", colorBold+colorGreen, colorReset)
	dataPkt := tunnel.NewDataPacket("conn-abc123", "stream-xyz789", []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	fmt.Printf("    %s包大小:%s %d 字节\n", colorDim, colorReset, len(dataPkt.Bytes()))
	printPacketDetail("  示例", dataPkt.Bytes(), "→")

	// 演示心跳包
	fmt.Printf("\n  %s心跳包 (Type=3):%s\n", colorBold+colorPurple, colorReset)
	hbPkt := tunnel.NewHeartbeatPacket("conn-abc123", 1, 0.0)
	fmt.Printf("    %s包大小:%s %d 字节\n", colorDim, colorReset, len(hbPkt.Bytes()))
	fmt.Printf("    %s包含:%s 时间戳(8B) + 序列号(4B) + 负载信息(4B)\n", colorDim, colorReset)

	// 演示分片
	fmt.Printf("\n  %s分片机制 (Type=6):%s\n", colorBold+colorCyan, colorReset)
	fmt.Printf("    %s阈值:%s 数据包 > %d 字节时触发分片\n", colorDim, colorReset, tunnel.MaxUDPPacketSize)
	fmt.Printf("    %s每个分片最大:%s %d 字节数据\n", colorDim, colorReset, tunnel.MaxFragmentDataSize)

	bigData := make([]byte, tunnel.MaxUDPPacketSize+1000)
	for i := range bigData {
		bigData[i] = byte(i % 256)
	}
	bigPkt := tunnel.NewDataPacket("conn-abc123", "stream-xyz789", bigData)
	fragments := tunnel.SplitPacket(&bigPkt.TunnelPacket)
	if fragments != nil {
		fmt.Printf("    %s示例:%s %d 字节数据 → %d 个分片\n", colorDim, colorReset, len(bigData), len(fragments))
		for i, f := range fragments {
			fmt.Printf("      %s分片 %d:%s 序列=%d 索引=%d/%d 数据=%d字节\n",
				colorDim, i+1, colorReset, f.SequenceID, f.FragmentIndex+1, f.TotalFragments, len(f.Data)-14)
		}
	}
}

// ============================================================
// 端到端实际通信演示
// ============================================================

func demoLiveCommunication(echoPort int) {
	nextStep("启动实际服务")

	// 1. 启动 SOCKS5 代理服务器 (UDP)
	serverPort := getFreePort()
	srvLogger := newDemoLogger("SERVER")
	srvConfig := server.Config{
		Port:     serverPort,
		LogLevel: common.DebugLevel,
	}
	srv := server.NewServer(srvConfig, srvLogger)
	if err := srv.Start(); err != nil {
		log.Fatalf("启动代理服务器失败: %v", err)
	}
	fmt.Printf("  %s✓ SOCKS5 代理服务器已启动 (UDP:%d)%s\n", colorGreen, serverPort, colorReset)

	// 2. 启动 SOCKS5 客户端 (TCP)
	clientPort := getFreePort()
	cliLogger := newDemoLogger("CLIENT")
	cliConfig := client.Config{
		LocalPort:  clientPort,
		ServerAddr: fmt.Sprintf("127.0.0.1:%d", serverPort),
		LogLevel:   common.DebugLevel,
		Timeout:    10 * time.Second,
	}
	cli := client.NewClient(cliConfig, cliLogger)
	if err := cli.Start(); err != nil {
		log.Fatalf("启动代理客户端失败: %v", err)
	}
	fmt.Printf("  %s✓ SOCKS5 代理客户端已启动 (TCP:%d → UDP:%d)%s\n", colorGreen, clientPort, serverPort, colorReset)

	time.Sleep(500 * time.Millisecond) // 等待握手完成

	// 3. 发送 HTTP 请求通过代理
	nextStep("通过代理链路发送 HTTP 请求")
	targetAddr := fmt.Sprintf("127.0.0.1:%d", echoPort)
	fmt.Printf("  %s请求链路:%s curl → SOCKS5(TCP:%d) → UDP Tunnel → SOCKS5(UDP:%d) → Echo(TCP:%d)\n",
		colorBold, colorReset, clientPort, serverPort, echoPort)
	fmt.Printf("  %s目标地址:%s %s\n\n", colorBold, colorReset, targetAddr)

	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", clientPort)
	url := fmt.Sprintf("http://%s/test-proxy-flow", targetAddr)

	// 使用自定义 HTTP 客户端（通过 SOCKS5 代理）
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 通过 SOCKS5 代理连接
			proxyConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", clientPort))
			if err != nil {
				return nil, err
			}

			// 手动执行 SOCKS5 握手
			// 1. 版本协商
			proxyConn.Write([]byte{0x05, 0x01, 0x00})
			resp := make([]byte, 2)
			io.ReadFull(proxyConn, resp)
			fmt.Printf("  %s[SOCKS5] 版本协商: 响应=[%02x %02x]%s\n", colorYellow, resp[0], resp[1], colorReset)

			// 2. CONNECT 请求
			host, portStr, _ := net.SplitHostPort(addr)
			port := 0
			fmt.Sscanf(portStr, "%d", &port)

			connectReq := buildSOCKS5Connect(host, port)
			proxyConn.Write(connectReq)
			fmt.Printf("  %s[SOCKS5] CONNECT 请求: %s:%d (ATYP=0x03 域名)%s\n", colorYellow, host, port, colorReset)

			connectResp := make([]byte, 10)
			io.ReadFull(proxyConn, connectResp)
			fmt.Printf("  %s[SOCKS5] CONNECT 响应: REP=%02x (%s)%s\n",
				colorGreen, connectResp[1], socks5RepText(connectResp[1]), colorReset)

			return proxyConn, nil
		},
	}

	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("  %s✗ HTTP 请求失败: %v%s\n", colorRed, err, colorReset)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("\n  %s✓ HTTP 响应 (状态码: %d)%s\n", colorGreen, resp.StatusCode, colorReset)
		fmt.Printf("  %s响应内容:%s\n%s%s%s\n", colorBold, colorReset, colorGreen, string(body), colorReset)
	}

	_ = proxyURL

	// 4. 等待一下看心跳
	nextStep("心跳保活机制")
	fmt.Printf("  %s心跳间隔:%s 1 秒\n", colorDim, colorReset)
	fmt.Printf("  %s观察:%s UDP 隧道上每隔 1 秒发送心跳包(Type=3)维持连接\n", colorDim, colorReset)
	fmt.Printf("  %s等待 3 秒观察心跳...%s\n", colorDim, colorReset)
	time.Sleep(3 * time.Second)

	// 5. 关闭
	nextStep("优雅关闭")
	cli.Stop()
	srv.Stop()
	fmt.Printf("  %s✓ 客户端已关闭%s\n", colorGreen, colorReset)
	fmt.Printf("  %s✓ 服务器已关闭%s\n", colorGreen, colorReset)
}

func buildSOCKS5Connect(host string, port int) []byte {
	buf := &bytes.Buffer{}
	buf.WriteByte(0x05) // VER
	buf.WriteByte(0x01) // CMD: CONNECT
	buf.WriteByte(0x00) // RSV

	// 域名类型
	buf.WriteByte(0x03) // ATYP: DOMAINNAME
	buf.WriteByte(byte(len(host)))
	buf.Write([]byte(host))
	binary.Write(buf, binary.BigEndian, uint16(port))

	return buf.Bytes()
}

func socks5RepText(rep byte) string {
	switch rep {
	case 0x00:
		return "成功"
	case 0x01:
		return "SOCKS服务器故障"
	case 0x02:
		return "不允许的连接"
	case 0x03:
		return "网络不可达"
	case 0x04:
		return "主机不可达"
	case 0x05:
		return "连接被拒绝"
	case 0x06:
		return "TTL超时"
	case 0x07:
		return "不支持的命令"
	case 0x08:
		return "不支持的地址类型"
	default:
		return fmt.Sprintf("未知错误(0x%02x)", rep)
	}
}

func getFreePort() int {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	ln, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("获取空闲端口失败: %v", err)
	}
	defer ln.Close()
	return ln.LocalAddr().(*net.UDPAddr).Port
}

// ============================================================
// 主函数
// ============================================================

func main() {
	printBanner()

	// 阶段 1: 协议分析演示
	demoSOCKS5Protocol("example.com:80")
	demoTunnelProtocol()

	// 阶段 2: 实际通信演示
	echoPort, echoCleanup := startEchoServer()
	defer echoCleanup()
	fmt.Printf("\n  %s✓ Echo 目标服务器已启动 (TCP:%d)%s\n", colorGreen, echoPort, colorReset)

	demoLiveCommunication(echoPort)

	// 总结
	nextStep("通信流程总结")
	fmt.Printf("\n  %s完整数据流转路径:%s\n\n", colorBold, colorReset)
	fmt.Printf("  %s1.%s curl 发送 HTTP 请求\n", colorGreen, colorReset)
	fmt.Printf("     %sGET /test-proxy-flow HTTP/1.1 → 127.0.0.1:%d (SOCKS5 Client)%s\n\n", colorDim, 1080, colorReset)

	fmt.Printf("  %s2.%s SOCKS5 Client 解析请求，提取目标地址\n", colorGreen, colorReset)
	fmt.Printf("     %s解析 SOCKS5 握手 → 获取目标域名/IP → 创建 UDP Tunnel Stream%s\n\n", colorDim, colorReset)

	fmt.Printf("  %s3.%s 通过 UDP Tunnel 发送 Tunnel Data Packet\n", colorGreen, colorReset)
	fmt.Printf("     %s封装为 [Header + StreamID + SOCKS5原始数据] → UDP → SOCKS5 Server%s\n\n", colorDim, colorReset)

	fmt.Printf("  %s4.%s SOCKS5 Server 接收并解包\n", colorGreen, colorReset)
	fmt.Printf("     %s解析 Tunnel Packet → 按 StreamID 找到/创建流 → 投递数据到 SOCKS5 处理%s\n\n", colorDim, colorReset)

	fmt.Printf("  %s5.%s Server 端 SOCKS5 处理器连接目标\n", colorGreen, colorReset)
	fmt.Printf("     %sgo-socks5 库执行 CONNECT → 与 Echo Server 建立 TCP 连接 → 转发数据%s\n\n", colorDim, colorReset)

	fmt.Printf("  %s6.%s 响应沿原路返回\n", colorGreen, colorReset)
	fmt.Printf("     %sEcho → SOCKS5 Server → UDP Tunnel → SOCKS5 Client → curl%s\n\n", colorDim, colorReset)

	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorCyan, colorReset)
	fmt.Printf("%s  演示完成! 按 Ctrl+C 退出%s\n", colorGreen, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorCyan, colorReset)

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Printf("\n%s再见!%s\n", colorGreen, colorReset)
}
