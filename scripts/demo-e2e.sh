#!/bin/bash
# UDP-SOCKS5 Proxy 端到端用户流程演示
# 从用户角度完整跑通：构建 → 启动 → 代理请求 → 关闭

set -e

# ── 颜色 ──
G='\033[32m' Y='\033[33m' B='\033[34m' C='\033[36m' R='\033[0m' BOLD='\033[1m' DIM='\033[2m'

cleanup() {
    echo -e "\n${DIM}清理进程...${R}"
    kill $ECHO_PID $SRV_PID $CLI_PID 2>/dev/null
    wait $ECHO_PID $SRV_PID $CLI_PID 2>/dev/null
    rm -f /tmp/echo_target.log /tmp/proxy_srv.log /tmp/proxy_cli.log
    echo -e "${G}✅ 已清理${R}"
}
trap cleanup EXIT

echo -e "${BOLD}${C}╔══════════════════════════════════════════════════════════════╗${R}"
echo -e "${BOLD}${C}║              UDP-SOCKS5 Proxy 端到端用户流程演示              ║${R}"
echo -e "${BOLD}${C}╚══════════════════════════════════════════════════════════════╝${R}"
echo ""

# ── 步骤 1: 构建 ──
echo -e "${BOLD}${G}━━━ 步骤 1: 构建二进制 ━━━${R}"
go build -o bin/server ./cmd/server/
go build -o bin/client ./cmd/client/
echo -e "${G}✅ 构建成功${R}"
ls -lh bin/server bin/client
echo ""

# ── 步骤 2: 启动 Echo 目标服务器 ──
echo -e "${BOLD}${G}━━━ 步骤 2: 启动 Echo 目标服务器 (模拟你要访问的网站) ━━━${R}"

# 写一个简单的目标服务器
cat > /tmp/echo_target.go << 'GOEOF'
package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	ln, _ := net.Listen("tcp", "127.0.0.1:"+os.Args[1])
	fmt.Printf("Echo 目标服务器启动: 127.0.0.1:%s\n", os.Args[1])
	for {
		conn, err := ln.Accept()
		if err != nil { return }
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 4096)
			n, _ := c.Read(buf)
			body := fmt.Sprintf("✅ Hello from Echo Server!\n🕐 Time: %s\n📥 Request first line: %s\n🔗 Proxy chain: curl → SOCKS5 Client → UDP Tunnel → SOCKS5 Server → Echo\n",
				time.Now().Format("2006-01-02 15:04:05.000"),
				func() string { 
					line := string(buf[:n])
					for i, ch := range line { if ch == '\r' || ch == '\n' { return line[:i] } }
					return line
				}())
			resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
			c.Write([]byte(resp))
		}(conn)
	}
}
GOEOF

go run /tmp/echo_target.go 7080 &>/tmp/echo_target.log &
ECHO_PID=$!
sleep 0.5
cat /tmp/echo_target.log
echo ""

# ── 步骤 3: 启动代理服务器 (UDP) ──
echo -e "${BOLD}${G}━━━ 步骤 3: 启动 SOCKS5 代理服务器 (UDP:9091) ━━━${R}"
./bin/server -port 9091 -log debug &>/tmp/proxy_srv.log &
SRV_PID=$!
sleep 0.5
cat /tmp/proxy_srv.log
echo ""

# ── 步骤 4: 启动代理客户端 (TCP) ──
echo -e "${BOLD}${G}━━━ 步骤 4: 启动 SOCKS5 代理客户端 (TCP:9090 → UDP:9091) ━━━${R}"
./bin/client -local 9090 -server 127.0.0.1:9091 -log debug &>/tmp/proxy_cli.log &
CLI_PID=$!
sleep 1
echo -e "${DIM}--- Client 日志 ---${R}"
cat /tmp/proxy_cli.log
echo ""
echo -e "${DIM}--- Server 日志 ---${R}"
cat /tmp/proxy_srv.log
echo ""

# ── 步骤 5: 通过代理发送请求 ──
echo -e "${BOLD}${G}━━━ 步骤 5: 通过 SOCKS5 代理发送 HTTP 请求 ━━━${R}"
echo -e "${Y}命令: curl -x socks5://127.0.0.1:9090 http://127.0.0.1:7080/hello-proxy${R}"
echo ""

RESULT=$(curl -s -x socks5://127.0.0.1:9090 http://127.0.0.1:7080/hello-proxy 2>&1)
CURL_EXIT=$?

if [ $CURL_EXIT -eq 0 ]; then
    echo -e "${G}✅ curl 成功！响应:${R}"
    echo -e "${G}${RESULT}${R}"
else
    echo -e "\033[31m✗ curl 失败 (exit=$CURL_EXIT):${R}"
    echo "$RESULT"
fi
echo ""

# ── 步骤 6: 查看通信日志 ──
sleep 1
echo -e "${BOLD}${G}━━━ 步骤 6: 通信日志 ━━━${R}"
echo -e "${C}${BOLD}── Client 端日志 (SOCKS5 处理 + Tunnel 发送) ──${R}"
cat /tmp/proxy_cli.log
echo ""
echo -e "${C}${BOLD}── Server 端日志 (Tunnel 接收 + SOCKS5 转发) ──${R}"
cat /tmp/proxy_srv.log
echo ""

# ── 步骤 7: 多发几个请求 ──
echo -e "${BOLD}${G}━━━ 步骤 7: 再发几个请求验证稳定性 ━━━${R}"

for i in 1 2 3; do
    echo -e "${Y}请求 $i: /test-$i${R}"
    RESP=$(curl -s -x socks5://127.0.0.1:9090 "http://127.0.0.1:7080/test-$i" 2>&1)
    if echo "$RESP" | grep -q "Hello from Echo"; then
        echo -e "  ${G}✅ 成功${R}"
    else
        echo -e "  \033[31m✗ 失败: $RESP${R}"
    fi
done
echo ""

# ── 步骤 8: 优雅关闭 ──
echo -e "${BOLD}${G}━━━ 步骤 8: 优雅关闭 ━━━${R}"
echo -e "${DIM}发送 SIGINT 给 client (PID=$CLI_PID)...${R}"
kill -INT $CLI_PID 2>/dev/null
sleep 0.5
echo -e "${DIM}发送 SIGINT 给 server (PID=$SRV_PID)...${R}"
kill -INT $SRV_PID 2>/dev/null
sleep 0.5
echo -e "${DIM}关闭 Echo 目标服务器 (PID=$ECHO_PID)...${R}"
kill $ECHO_PID 2>/dev/null
sleep 0.3
echo -e "${G}✅ 全部服务已关闭${R}"
echo ""

echo -e "${BOLD}${C}╔══════════════════════════════════════════════════════════════╗${R}"
echo -e "${BOLD}${C}║                    演示完成！                                 ║${R}"
echo -e "${BOLD}${C}╚══════════════════════════════════════════════════════════════╝${R}"
