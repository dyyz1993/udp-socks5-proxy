.PHONY: all clean client server test

# 输出目录
BIN_DIR=bin

# 二进制文件名
CLIENT_BIN=$(BIN_DIR)/client
SERVER_BIN=$(BIN_DIR)/server

# Go命令
GO=go
GOBUILD=$(GO) build
GOTEST=$(GO) test
GOCLEAN=$(GO) clean

# 编译标志
LDFLAGS=-ldflags "-s -w"

# 默认目标
all: client server

# 创建输出目录
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# 编译客户端
client: $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(CLIENT_BIN) cmd/client/main.go

# 编译服务端
server: $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SERVER_BIN) cmd/server/main.go

# 执行测试
test:
	$(GOTEST) -v ./...

# 清理构建产物
clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)

# 安装依赖
deps:
	$(GO) mod tidy 