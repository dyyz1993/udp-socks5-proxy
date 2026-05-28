#!/bin/bash

# 设置颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # 重置颜色

echo -e "${YELLOW}开始执行单个测试...${NC}"

# 逐个运行测试，确保每个测试有足够的超时时间
go test -v -timeout 15s -run TestFragmentPacket ./src/testing/ && \
echo -e "${GREEN}TestFragmentPacket 测试通过${NC}" || \
echo -e "${RED}TestFragmentPacket 测试失败${NC}"

go test -v -timeout 15s -run TestFragmentReassembly ./src/testing/ && \
echo -e "${GREEN}TestFragmentReassembly 测试通过${NC}" || \
echo -e "${RED}TestFragmentReassembly 测试失败${NC}"

go test -v -timeout 15s -run TestMultiClientHandshakeAndHeartbeat ./src/testing/ && \
echo -e "${GREEN}TestMultiClientHandshakeAndHeartbeat 测试通过${NC}" || \
echo -e "${RED}TestMultiClientHandshakeAndHeartbeat 测试失败${NC}"

go test -v -timeout 15s -run TestHeartbeatStability ./src/testing/ && \
echo -e "${GREEN}TestHeartbeatStability 测试通过${NC}" || \
echo -e "${RED}TestHeartbeatStability 测试失败${NC}" 