package tunnel

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTunnelPacketBytes 测试基础数据包的序列化
func TestTunnelPacketBytes(t *testing.T) {
	connID := "test-connection-123"
	testData := []byte("hello, world")

	packet := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeData,
			Flags:        0,
			ConnectionID: connID,
		},
		Data: testData,
	}

	// 序列化
	packetBytes := packet.Bytes()

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if parsedPacket.Header.Version != ProtocolVersion {
		t.Errorf("版本不匹配: got %d, want %d", parsedPacket.Header.Version, ProtocolVersion)
	}

	if parsedPacket.Header.Type != PacketTypeData {
		t.Errorf("包类型不匹配: got %d, want %d", parsedPacket.Header.Type, PacketTypeData)
	}

	if parsedPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", parsedPacket.Header.ConnectionID, connID)
	}

	if !bytes.Equal(parsedPacket.Data, testData) {
		t.Errorf("数据不匹配: got %v, want %v", parsedPacket.Data, testData)
	}
}

// TestParsePacketInvalidData 测试解析无效数据包
func TestParsePacketInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"太短的数据", []byte{1, 2, 3}},
		{"ID长度无效", []byte{1, 2, 3, 255, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePacket(tt.data)
			if err == nil {
				t.Error("应该返回错误，但未返回")
			}
			if err != ErrInvalidPacket {
				t.Errorf("错误类型不匹配: got %v, want %v", err, ErrInvalidPacket)
			}
		})
	}
}

// TestHandshakePacket 测试握手包创建和序列化
func TestHandshakePacket(t *testing.T) {
	connID := "handshake-test-123"
	var key [32]byte
	copy(key[:], "test-key-12345678901234567890123")
	group := "default"
	features := uint32(123)
	version := "1.0.0"

	// 创建握手包
	packet := NewHandshakePacket(connID, key, group, features, version)

	// 序列化
	packetBytes := packet.Bytes()

	// 验证类型
	if packet.Type() != PacketTypeHandshake {
		t.Errorf("包类型不匹配: got %d, want %d", packet.Type(), PacketTypeHandshake)
	}

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if parsedPacket.Header.Type != PacketTypeHandshake {
		t.Errorf("包类型不匹配: got %d, want %d", parsedPacket.Header.Type, PacketTypeHandshake)
	}

	if parsedPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", parsedPacket.Header.ConnectionID, connID)
	}
}

// TestDataPacket 测试数据包创建和序列化
func TestDataPacket(t *testing.T) {
	connID := "test-connection-123"
	streamID := "test-stream-456"
	testData := []byte("hello, world")

	// 创建数据包
	packet := NewDataPacket(connID, streamID, testData)

	// 验证字段
	if packet.Header.Type != PacketTypeData {
		t.Errorf("包类型不匹配: got %d, want %d", packet.Header.Type, PacketTypeData)
	}

	if packet.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", packet.Header.ConnectionID, connID)
	}

	if packet.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", packet.Header.StreamID, streamID)
	}

	// 序列化
	packetBytes := packet.Bytes()

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	dataPacket, err := ParseDataPacket(parsedPacket)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if dataPacket.Header.Type != PacketTypeData {
		t.Errorf("包类型不匹配: got %d, want %d", dataPacket.Header.Type, PacketTypeData)
	}

	if dataPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", dataPacket.Header.ConnectionID, connID)
	}

	if dataPacket.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", dataPacket.Header.StreamID, streamID)
	}

	// 验证数据是否正确提取（现在数据包不再包含StreamID）
	if !bytes.Equal(dataPacket.Data, testData) {
		t.Errorf("数据不匹配: got %v, want %v", dataPacket.Data, testData)
	}
}

// TestHeartbeatPacket 测试心跳包创建和序列化
func TestHeartbeatPacket(t *testing.T) {
	connID := "heartbeat-test-123"
	sequence := uint32(123)
	load := float32(0.75)

	// 创建心跳包
	packet := NewHeartbeatPacket(connID, sequence, load)

	// 验证类型
	if packet.Type() != PacketTypeHeartbeat {
		t.Errorf("包类型不匹配: got %d, want %d", packet.Type(), PacketTypeHeartbeat)
	}

	// 验证字段
	if packet.Sequence != sequence {
		t.Errorf("序列号不匹配: got %d, want %d", packet.Sequence, sequence)
	}

	if packet.Load != load {
		t.Errorf("负载不匹配: got %f, want %f", packet.Load, load)
	}

	// 序列化
	packetBytes := packet.Bytes()

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if parsedPacket.Header.Type != PacketTypeHeartbeat {
		t.Errorf("包类型不匹配: got %d, want %d", parsedPacket.Header.Type, PacketTypeHeartbeat)
	}

	if parsedPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", parsedPacket.Header.ConnectionID, connID)
	}
}

// TestClosePacket 测试关闭包创建和序列化
func TestClosePacket(t *testing.T) {
	connID := "test-connection-123"
	streamID := "test-stream-456"

	// 创建关闭包
	packet := NewClosePacket(connID, streamID)

	// 验证字段
	if packet.Header.Type != PacketTypeClose {
		t.Errorf("包类型不匹配: got %d, want %d", packet.Header.Type, PacketTypeClose)
	}

	if packet.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", packet.Header.ConnectionID, connID)
	}

	if packet.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", packet.Header.StreamID, streamID)
	}

	// 序列化
	packetBytes := packet.Bytes()

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析关闭包失败: %v", err)
	}

	closePacket, err := ParseClosePacket(parsedPacket)
	if err != nil {
		t.Fatalf("解析关闭包失败: %v", err)
	}

	// 验证字段
	if closePacket.Header.Type != PacketTypeClose {
		t.Errorf("包类型不匹配: got %d, want %d", closePacket.Header.Type, PacketTypeClose)
	}

	if closePacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", closePacket.Header.ConnectionID, connID)
	}

	if closePacket.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", closePacket.Header.StreamID, streamID)
	}
}

// TestErrorPacket 测试错误包创建和序列化
func TestErrorPacket(t *testing.T) {
	connID := "error-test-123"
	code := ErrCodeAuthFailed
	message := "认证失败"
	relatedID := "related-stream-123"

	// 创建错误包
	packet := NewErrorPacket(connID, code, message, relatedID)

	// 验证类型
	if packet.Type() != PacketTypeError {
		t.Errorf("包类型不匹配: got %d, want %d", packet.Type(), PacketTypeError)
	}

	// 验证字段
	if packet.Code != code {
		t.Errorf("错误码不匹配: got %d, want %d", packet.Code, code)
	}

	if packet.Message != message {
		t.Errorf("错误信息不匹配: got %s, want %s", packet.Message, message)
	}

	if packet.RelatedID != relatedID {
		t.Errorf("相关ID不匹配: got %s, want %s", packet.RelatedID, relatedID)
	}

	// 序列化
	packetBytes := packet.Bytes()

	// 反序列化
	parsedPacket, err := ParsePacket(packetBytes)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if parsedPacket.Header.Type != PacketTypeError {
		t.Errorf("包类型不匹配: got %d, want %d", parsedPacket.Header.Type, PacketTypeError)
	}

	if parsedPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", parsedPacket.Header.ConnectionID, connID)
	}
}

// TestGenerateIDs 测试ID生成方法
func TestGenerateIDs(t *testing.T) {
	// 测试连接ID生成
	connID := GenerateConnectionID()
	if connID == "" {
		t.Error("生成的连接ID不应为空")
	}

	// 测试流ID生成
	streamID := GenerateStreamID()
	if streamID == "" {
		t.Error("生成的流ID不应为空")
	}

	// 测试ID唯一性
	anotherConnID := GenerateConnectionID()
	if connID == anotherConnID {
		t.Error("两次生成的连接ID应不同")
	}

	anotherStreamID := GenerateStreamID()
	if streamID == anotherStreamID {
		t.Error("两次生成的流ID应不同")
	}
}

// TestNewDataPacket 测试创建数据包
func TestNewDataPacket(t *testing.T) {
	connectionID := "test-conn-id"
	streamID := "test-stream-id"
	testData := []byte("test data payload")

	packet := NewDataPacket(connectionID, streamID, testData)

	if packet.Header.Version != ProtocolVersion {
		t.Errorf("版本号错误, 期望: %d, 实际: %d", ProtocolVersion, packet.Header.Version)
	}

	if packet.Header.Type != PacketTypeData {
		t.Errorf("包类型错误, 期望: %d, 实际: %d", PacketTypeData, packet.Header.Type)
	}

	if packet.Header.ConnectionID != connectionID {
		t.Errorf("连接ID错误, 期望: %s, 实际: %s", connectionID, packet.Header.ConnectionID)
	}

	if packet.Header.StreamID != streamID {
		t.Errorf("流ID错误, 期望: %s, 实际: %s", streamID, packet.Header.StreamID)
	}

	// 验证序列化和反序列化
	rawData := packet.Bytes()
	parsedPacket, err := ParsePacket(rawData)

	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	if parsedPacket.Header.Version != packet.Header.Version {
		t.Error("解析后版本号不匹配")
	}

	if parsedPacket.Header.Type != packet.Header.Type {
		t.Error("解析后包类型不匹配")
	}

	if parsedPacket.Header.ConnectionID != packet.Header.ConnectionID {
		t.Error("解析后连接ID不匹配")
	}
}

// TestNewHeartbeatPacket 测试创建心跳包
func TestNewHeartbeatPacket(t *testing.T) {
	connectionID := "test-conn-id"
	sequence := uint32(123)
	load := float32(0.75)

	packet := NewHeartbeatPacket(connectionID, sequence, load)

	if packet.Header.Version != ProtocolVersion {
		t.Errorf("版本号错误, 期望: %d, 实际: %d", ProtocolVersion, packet.Header.Version)
	}

	if packet.Header.Type != PacketTypeHeartbeat {
		t.Errorf("包类型错误, 期望: %d, 实际: %d", PacketTypeHeartbeat, packet.Header.Type)
	}

	if packet.Header.ConnectionID != connectionID {
		t.Errorf("连接ID错误, 期望: %s, 实际: %s", connectionID, packet.Header.ConnectionID)
	}

	if packet.Sequence != sequence {
		t.Errorf("序列号错误, 期望: %d, 实际: %d", sequence, packet.Sequence)
	}

	if packet.Load != load {
		t.Errorf("负载值错误, 期望: %f, 实际: %f", load, packet.Load)
	}

	// 验证心跳包的时间戳是否合理（在过去5秒内创建的）
	now := time.Now().UnixNano()
	if now-packet.Timestamp > int64(5*time.Second) {
		t.Error("心跳包时间戳不合理")
	}

	// 验证序列化和反序列化
	rawData := packet.Bytes()
	_, err := ParsePacket(rawData)

	if err != nil {
		t.Fatalf("解析心跳包失败: %v", err)
	}
}

// TestNewClosePacket 测试创建关闭包
func TestNewClosePacket(t *testing.T) {
	connectionID := "test-conn-id"
	streamID := "test-stream-id"

	packet := NewClosePacket(connectionID, streamID)

	if packet.Header.Version != ProtocolVersion {
		t.Errorf("版本号错误, 期望: %d, 实际: %d", ProtocolVersion, packet.Header.Version)
	}

	if packet.Header.Type != PacketTypeClose {
		t.Errorf("包类型错误, 期望: %d, 实际: %d", PacketTypeClose, packet.Header.Type)
	}

	if packet.Header.ConnectionID != connectionID {
		t.Errorf("连接ID错误, 期望: %s, 实际: %s", connectionID, packet.Header.ConnectionID)
	}

	if packet.Header.StreamID != streamID {
		t.Errorf("流ID错误, 期望: %s, 实际: %s", streamID, packet.Header.StreamID)
	}

	// 验证序列化和反序列化
	rawData := packet.Bytes()
	parsedPacket, err := ParsePacket(rawData)

	if err != nil {
		t.Fatalf("解析关闭包失败: %v", err)
	}

	if parsedPacket.Header.Type != PacketTypeClose {
		t.Error("解析后包类型不匹配")
	}
}

// TestNewErrorPacket 测试创建错误包
func TestNewErrorPacket(t *testing.T) {
	connectionID := "test-conn-id"
	code := 404
	message := "not found"
	relatedID := "test-stream-id"

	packet := NewErrorPacket(connectionID, code, message, relatedID)

	if packet.Header.Version != ProtocolVersion {
		t.Errorf("版本号错误, 期望: %d, 实际: %d", ProtocolVersion, packet.Header.Version)
	}

	if packet.Header.Type != PacketTypeError {
		t.Errorf("包类型错误, 期望: %d, 实际: %d", PacketTypeError, packet.Header.Type)
	}

	if packet.Header.ConnectionID != connectionID {
		t.Errorf("连接ID错误, 期望: %s, 实际: %s", connectionID, packet.Header.ConnectionID)
	}

	if packet.Code != code {
		t.Errorf("错误码错误, 期望: %d, 实际: %d", code, packet.Code)
	}

	if packet.Message != message {
		t.Errorf("错误消息错误, 期望: %s, 实际: %s", message, packet.Message)
	}

	if packet.RelatedID != relatedID {
		t.Errorf("相关ID错误, 期望: %s, 实际: %s", relatedID, packet.RelatedID)
	}

	// 验证序列化和反序列化
	rawData := packet.Bytes()
	parsedPacket, err := ParsePacket(rawData)

	if err != nil {
		t.Fatalf("解析错误包失败: %v", err)
	}

	if parsedPacket.Header.Type != PacketTypeError {
		t.Error("解析后包类型不匹配")
	}
}

// TestNewHandshakePacket 测试创建握手包
func TestNewHandshakePacket(t *testing.T) {
	connectionID := "test-conn-id"
	var key [32]byte
	copy(key[:], "test-handshake-key-0123456789abcdef")
	group := "test-group"
	features := uint32(0x01020304)
	version := "1.0.0"

	packet := NewHandshakePacket(connectionID, key, group, features, version)

	if packet.Header.Version != ProtocolVersion {
		t.Errorf("版本号错误, 期望: %d, 实际: %d", ProtocolVersion, packet.Header.Version)
	}

	if packet.Header.Type != PacketTypeHandshake {
		t.Errorf("包类型错误, 期望: %d, 实际: %d", PacketTypeHandshake, packet.Header.Type)
	}

	if packet.Header.ConnectionID != connectionID {
		t.Errorf("连接ID错误, 期望: %s, 实际: %s", connectionID, packet.Header.ConnectionID)
	}

	if !bytes.Equal(packet.Key[:], key[:]) {
		t.Error("密钥不匹配")
	}

	if packet.Group != group {
		t.Errorf("分组错误, 期望: %s, 实际: %s", group, packet.Group)
	}

	if packet.Features != features {
		t.Errorf("特性标志错误, 期望: %d, 实际: %d", features, packet.Features)
	}

	if packet.Version != version {
		t.Errorf("版本错误, 期望: %s, 实际: %s", version, packet.Version)
	}

	// 验证序列化和反序列化
	rawData := packet.Bytes()
	parsedPacket, err := ParsePacket(rawData)

	if err != nil {
		t.Fatalf("解析握手包失败: %v", err)
	}

	if parsedPacket.Header.Type != PacketTypeHandshake {
		t.Error("解析后包类型不匹配")
	}
}

// TestParsePacket 测试解析不同类型的数据包
func TestParsePacket(t *testing.T) {
	// 数据包
	dataPacket := NewDataPacket("conn-id-1", "stream-id-1", []byte("test data"))
	dataBytes := dataPacket.Bytes()

	parsed, err := ParsePacket(dataBytes)
	if err != nil {
		t.Errorf("解析数据包失败: %v", err)
	}
	if parsed.Header.Type != PacketTypeData {
		t.Errorf("解析数据包类型错误: %v", parsed.Header.Type)
	}

	// 心跳包
	heartbeatPacket := NewHeartbeatPacket("conn-id-2", 1, 0.5)
	heartbeatBytes := heartbeatPacket.Bytes()

	parsed, err = ParsePacket(heartbeatBytes)
	if err != nil {
		t.Errorf("解析心跳包失败: %v", err)
	}
	if parsed.Header.Type != PacketTypeHeartbeat {
		t.Errorf("解析心跳包类型错误: %v", parsed.Header.Type)
	}

	// 关闭包
	closePacket := NewClosePacket("conn-id-3", "stream-id-3")
	closeBytes := closePacket.Bytes()

	parsed, err = ParsePacket(closeBytes)
	if err != nil {
		t.Errorf("解析关闭包失败: %v", err)
	}
	if parsed.Header.Type != PacketTypeClose {
		t.Errorf("解析关闭包类型错误: %v", parsed.Header.Type)
	}

	// 错误包
	errorPacket := NewErrorPacket("conn-id-4", 500, "internal error", "stream-id-4")
	errorBytes := errorPacket.Bytes()

	parsed, err = ParsePacket(errorBytes)
	if err != nil {
		t.Errorf("解析错误包失败: %v", err)
	}
	if parsed.Header.Type != PacketTypeError {
		t.Errorf("解析错误包类型错误: %v", parsed.Header.Type)
	}
}

// TestParseInvalidPacket 测试解析无效的数据包
func TestParseInvalidPacket(t *testing.T) {
	// 太短的数据包
	_, err := ParsePacket([]byte{0x01, 0x02})
	if err == nil {
		t.Error("应该返回错误但没有")
	}

	// 无效的连接ID长度
	invalidData := []byte{
		0x01, 0x02, 0x00, // 版本, 类型, 标志
		0xFF, 0xFF, // 连接ID长度 (65535，太长了)
	}
	_, err = ParsePacket(invalidData)
	if err == nil {
		t.Error("处理无效的连接ID长度应返回错误")
	}
}

// TestParseDataPacket 测试从基础数据包解析数据包
func TestParseDataPacket(t *testing.T) {
	// 创建基础数据包
	connID := "test-connection-123"
	streamID := "test-stream-456"
	testData := []byte("hello, world")
	packet := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeData,
			Flags:        0,
			ConnectionID: connID,
			StreamID:     streamID,
		},
		Data: testData,
	}

	// 解析数据包
	dataPacket, err := ParseDataPacket(packet)
	if err != nil {
		t.Fatalf("解析数据包失败: %v", err)
	}

	// 验证字段
	if dataPacket.Header.Type != PacketTypeData {
		t.Errorf("包类型不匹配: got %d, want %d", dataPacket.Header.Type, PacketTypeData)
	}

	if dataPacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", dataPacket.Header.ConnectionID, connID)
	}

	if dataPacket.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", dataPacket.Header.StreamID, streamID)
	}

	// 验证数据
	if !bytes.Equal(dataPacket.Data, testData) {
		t.Errorf("数据不匹配: got %v, want %v", dataPacket.Data, testData)
	}
}

// TestParseClosePacket 测试从基础数据包解析关闭包
func TestParseClosePacket(t *testing.T) {
	// 创建基础数据包
	connID := "test-connection-123"
	streamID := "test-stream-456"
	packet := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeClose,
			Flags:        0,
			ConnectionID: connID,
			StreamID:     streamID,
		},
		Data: nil,
	}

	// 解析关闭包
	closePacket, err := ParseClosePacket(packet)
	if err != nil {
		t.Fatalf("解析关闭包失败: %v", err)
	}

	// 验证字段
	if closePacket.Header.Type != PacketTypeClose {
		t.Errorf("包类型不匹配: got %d, want %d", closePacket.Header.Type, PacketTypeClose)
	}

	if closePacket.Header.ConnectionID != connID {
		t.Errorf("连接ID不匹配: got %s, want %s", closePacket.Header.ConnectionID, connID)
	}

	if closePacket.Header.StreamID != streamID {
		t.Errorf("流ID不匹配: got %s, want %s", closePacket.Header.StreamID, streamID)
	}
}

func TestFragmentPacket_Creation(t *testing.T) {
	// 测试数据
	connectionID := "test-connection"
	streamID := "test-stream"
	originalType := PacketType(PacketTypeData)
	sequenceID := uint32(12345)
	totalFragments := uint32(3)
	fragmentIndex := uint32(1)
	flags := uint8(FragmentFlagMore)
	fragmentData := []byte("这是测试数据")

	// 创建分片包
	packet := NewFragmentPacket(
		connectionID,
		streamID,
		originalType,
		sequenceID,
		totalFragments,
		fragmentIndex,
		flags,
		fragmentData,
	)

	// 验证分片包属性
	assert.Equal(t, PacketType(PacketTypeFragmented), PacketType(packet.Header.Type), "分片包类型应该是Fragmented")
	assert.Equal(t, connectionID, packet.Header.ConnectionID, "连接ID不匹配")
	assert.Equal(t, streamID, packet.Header.StreamID, "流ID不匹配")
	assert.Equal(t, sequenceID, packet.SequenceID, "序列ID不匹配")
	assert.Equal(t, totalFragments, packet.TotalFragments, "总分片数不匹配")
	assert.Equal(t, fragmentIndex, packet.FragmentIndex, "分片索引不匹配")
	assert.Equal(t, flags, packet.Flags, "分片标记不匹配")
	assert.Equal(t, PacketType(originalType), PacketType(packet.OriginalType), "原始类型不匹配")

	// 测试GetFragmentData方法
	data := packet.GetFragmentData()
	assert.Equal(t, fragmentData, data, "获取的分片数据不匹配")
}

func TestParseFragmentPacket(t *testing.T) {
	// 创建一个原始分片包
	connectionID := "test-connection"
	streamID := "test-stream"
	originalType := PacketType(PacketTypeData)
	sequenceID := uint32(12345)
	totalFragments := uint32(3)
	fragmentIndex := uint32(1)
	flags := uint8(FragmentFlagMore)
	fragmentData := []byte("这是测试数据")

	originalPacket := NewFragmentPacket(
		connectionID,
		streamID,
		originalType,
		sequenceID,
		totalFragments,
		fragmentIndex,
		flags,
		fragmentData,
	)

	// 获取分片包的字节表示
	packetBytes := originalPacket.Bytes()

	// 解析基础包
	basePacket, err := ParsePacket(packetBytes)
	require.NoError(t, err, "解析基础包失败")

	// 测试解析分片包
	parsedPacket, err := ParseFragmentPacket(basePacket)
	require.NoError(t, err, "解析分片包失败")

	// 验证解析后的分片包
	assert.Equal(t, PacketType(PacketTypeFragmented), PacketType(parsedPacket.Header.Type), "分片包类型应该是Fragmented")
	assert.Equal(t, connectionID, parsedPacket.Header.ConnectionID, "连接ID不匹配")
	assert.Equal(t, streamID, parsedPacket.Header.StreamID, "流ID不匹配")
	assert.Equal(t, sequenceID, parsedPacket.SequenceID, "序列ID不匹配")
	assert.Equal(t, totalFragments, parsedPacket.TotalFragments, "总分片数不匹配")
	assert.Equal(t, fragmentIndex, parsedPacket.FragmentIndex, "分片索引不匹配")
	assert.Equal(t, flags, parsedPacket.Flags, "分片标记不匹配")
	assert.Equal(t, PacketType(originalType), PacketType(parsedPacket.OriginalType), "原始类型不匹配")

	// 测试GetFragmentData方法
	data := parsedPacket.GetFragmentData()
	assert.Equal(t, fragmentData, data, "获取的分片数据不匹配")

	// 测试解析错误情况
	badPacket := &TunnelPacket{
		Header: Header{
			Type: PacketTypeData, // 错误的包类型
		},
	}
	_, err = ParseFragmentPacket(badPacket)
	assert.Error(t, err, "对非分片包应该返回错误")

	// 测试数据包太短的情况
	shortPacket := &TunnelPacket{
		Header: Header{
			Type: PacketTypeFragmented,
		},
		Data: []byte{1, 2, 3}, // 太短
	}
	_, err = ParseFragmentPacket(shortPacket)
	assert.Error(t, err, "数据包太短应该返回错误")
}

func TestSplitAndMergePacket(t *testing.T) {
	// 测试数据
	connectionID := "test-connection"
	streamID := "test-stream"

	// 创建需要分片的大数据包
	largeData := make([]byte, MaxUDPPacketSize+1000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// 创建原始数据包
	originalPacket := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeData,
			ConnectionID: connectionID,
			StreamID:     streamID,
		},
		Data: largeData,
	}

	// 测试分片
	fragments := SplitPacket(originalPacket)
	assert.NotNil(t, fragments, "应该产生分片")

	// 验证分片数量
	expectedFragments := (len(largeData) + MaxFragmentDataSize - 1) / MaxFragmentDataSize
	assert.Equal(t, expectedFragments, len(fragments), "分片数量不正确")

	// 验证分片属性
	for i, fragment := range fragments {
		assert.Equal(t, connectionID, fragment.Header.ConnectionID, "分片连接ID应该匹配")
		assert.Equal(t, streamID, fragment.Header.StreamID, "分片流ID应该匹配")
		assert.Equal(t, PacketType(PacketTypeFragmented), PacketType(fragment.Header.Type), "分片类型应该是Fragmented")
		assert.Equal(t, PacketType(PacketTypeData), PacketType(fragment.OriginalType), "原始类型应该是Data")
		assert.Equal(t, uint32(i), fragment.FragmentIndex, "分片索引应该匹配")
		assert.Equal(t, uint32(len(fragments)), fragment.TotalFragments, "总分片数应该匹配")

		// 验证分片标记
		if i == 0 {
			assert.True(t, (fragment.Flags&FragmentFlagStart) != 0, "首片应该有Start标记")
		}
		if i == len(fragments)-1 {
			assert.True(t, (fragment.Flags&FragmentFlagEnd) != 0, "末片应该有End标记")
		}
		if i < len(fragments)-1 {
			assert.True(t, (fragment.Flags&FragmentFlagMore) != 0, "非末片应该有More标记")
		}
	}

	// 测试合并
	mergedPacket, err := MergeFragments(fragments)
	require.NoError(t, err, "合并分片应该成功")

	// 验证合并后的包
	assert.Equal(t, connectionID, mergedPacket.Header.ConnectionID, "合并包连接ID应该匹配")
	assert.Equal(t, streamID, mergedPacket.Header.StreamID, "合并包流ID应该匹配")
	assert.Equal(t, PacketType(PacketTypeData), PacketType(mergedPacket.Header.Type), "合并包类型应该是原始类型")
	assert.Equal(t, len(largeData), len(mergedPacket.Data), "合并包数据长度应该匹配")
	assert.True(t, bytes.Equal(largeData, mergedPacket.Data), "合并包数据应该匹配原始数据")

	// 测试合并错误情况

	// 1. 空分片列表
	_, err = MergeFragments([]*FragmentPacket{})
	assert.Error(t, err, "合并空分片列表应该返回错误")

	// 2. 分片顺序错误
	if len(fragments) > 1 {
		shuffled := make([]*FragmentPacket, len(fragments))
		copy(shuffled, fragments)
		// 交换第一个和最后一个分片
		shuffled[0], shuffled[len(shuffled)-1] = shuffled[len(shuffled)-1], shuffled[0]
		_, err = MergeFragments(shuffled)
		assert.Error(t, err, "合并乱序分片应该返回错误")
	}

	// 3. 连接ID不匹配
	if len(fragments) > 1 {
		mismatchedConn := make([]*FragmentPacket, len(fragments))
		copy(mismatchedConn, fragments)
		mismatchedConn[1].Header.ConnectionID = "different-conn"
		_, err = MergeFragments(mismatchedConn)
		assert.Error(t, err, "连接ID不匹配应该返回错误")
	}

	// 4. 流ID不匹配
	if len(fragments) > 1 {
		mismatchedStream := make([]*FragmentPacket, len(fragments))
		copy(mismatchedStream, fragments)
		mismatchedStream[1].Header.StreamID = "different-stream"
		_, err = MergeFragments(mismatchedStream)
		assert.Error(t, err, "流ID不匹配应该返回错误")
	}
}

func TestSplitPacket_Small(t *testing.T) {
	// 创建小数据包（不需要分片）
	smallData := make([]byte, 100)
	for i := range smallData {
		smallData[i] = byte(i % 256)
	}

	packet := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeData,
			ConnectionID: "test-conn",
			StreamID:     "test-stream",
		},
		Data: smallData,
	}

	// 确保小数据包不会被分片
	fragments := SplitPacket(packet)
	assert.Nil(t, fragments, "小数据包不应该被分片")
}

func TestFragmentPacket_Bytes(t *testing.T) {
	// 创建分片包
	fragment := NewFragmentPacket(
		"test-conn",
		"test-stream",
		PacketTypeData,
		12345,
		3,
		1,
		FragmentFlagMore,
		[]byte("test data"),
	)

	// 获取字节表示
	bytes := fragment.Bytes()

	// 应该能够解析回来
	parsedBase, err := ParsePacket(bytes)
	require.NoError(t, err, "解析字节应该成功")

	parsed, err := ParseFragmentPacket(parsedBase)
	require.NoError(t, err, "解析分片包应该成功")

	assert.Equal(t, fragment.SequenceID, parsed.SequenceID, "序列ID应该匹配")
	assert.Equal(t, fragment.FragmentIndex, parsed.FragmentIndex, "分片索引应该匹配")
}

func TestMergeFragments_MismatchedSequence(t *testing.T) {
	// 创建两个分片，序列ID不同
	fragments := []*FragmentPacket{
		NewFragmentPacket("test-conn", "test-stream", PacketTypeData, 1, 2, 0, FragmentFlagStart|FragmentFlagMore, []byte("data1")),
		NewFragmentPacket("test-conn", "test-stream", PacketTypeData, 2, 2, 1, FragmentFlagEnd, []byte("data2")),
	}

	_, err := MergeFragments(fragments)
	assert.Error(t, err, "序列ID不匹配应该返回错误")
}

func TestMergeFragments_MismatchedTotal(t *testing.T) {
	// 创建两个分片，总数不同
	fragments := []*FragmentPacket{
		NewFragmentPacket("test-conn", "test-stream", PacketTypeData, 1, 2, 0, FragmentFlagStart|FragmentFlagMore, []byte("data1")),
		NewFragmentPacket("test-conn", "test-stream", PacketTypeData, 1, 3, 1, FragmentFlagMore, []byte("data2")),
	}

	_, err := MergeFragments(fragments)
	assert.Error(t, err, "总分片数不匹配应该返回错误")
}

func TestMergeFragments_MismatchedType(t *testing.T) {
	// 创建两个分片，原始类型不同
	fragments := []*FragmentPacket{
		NewFragmentPacket("test-conn", "test-stream", PacketTypeData, 1, 2, 0, FragmentFlagStart|FragmentFlagMore, []byte("data1")),
		NewFragmentPacket("test-conn", "test-stream", PacketTypeClose, 1, 2, 1, FragmentFlagEnd, []byte("data2")),
	}

	_, err := MergeFragments(fragments)
	assert.Error(t, err, "原始类型不匹配应该返回错误")
}

func TestSplitPacket_GeneratesUniqueSequenceIDs(t *testing.T) {
	// 创建数据包
	packet := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         PacketTypeData,
			ConnectionID: "test-conn",
			StreamID:     "test-stream",
		},
		Data: make([]byte, MaxUDPPacketSize+1000),
	}

	// 分片两次，应该生成不同的序列ID
	fragments1 := SplitPacket(packet)
	time.Sleep(1 * time.Millisecond) // 确保时间戳不同
	fragments2 := SplitPacket(packet)

	assert.NotEqual(t, fragments1[0].SequenceID, fragments2[0].SequenceID, "不同时间的分片应该有不同的序列ID")
}

func TestParseErrorPacket(t *testing.T) {
	// Create a valid error packet
	errPkt := NewErrorPacket("conn1", 1001, "test error", "stream1")
	raw := errPkt.Bytes()

	// Parse it back
	tunnelPkt, err := ParsePacket(raw)
	require.NoError(t, err)

	parsed, err := ParseErrorPacket(tunnelPkt)
	require.NoError(t, err)
	assert.Equal(t, 1001, parsed.Code)
	assert.Equal(t, "test error", parsed.Message)
	assert.Equal(t, "stream1", parsed.RelatedID)
}

func TestParseErrorPacket_WrongType(t *testing.T) {
	dataPkt := &TunnelPacket{
		Header: Header{Type: PacketTypeData},
		Data:   []byte("hello"),
	}
	_, err := ParseErrorPacket(dataPkt)
	assert.Error(t, err)
}

func TestParseErrorPacket_InvalidData(t *testing.T) {
	tunnelPkt := &TunnelPacket{
		Header: Header{Type: PacketTypeError},
		Data:   []byte{0x01}, // too short
	}
	_, err := ParseErrorPacket(tunnelPkt)
	assert.Error(t, err)
}

func TestParseHandshakePacket(t *testing.T) {
	key := [32]byte{}
	for i := range key {
		key[i] = byte(i)
	}

	hsPkt := NewHandshakePacket("conn1", key, "test-group", 42, "2.0")
	raw := hsPkt.Bytes()

	tunnelPkt, err := ParsePacket(raw)
	require.NoError(t, err)

	parsed, err := ParseHandshakePacket(tunnelPkt)
	require.NoError(t, err)
	assert.Equal(t, "conn1", parsed.Header.ConnectionID)
	assert.Equal(t, key, parsed.Key)
	assert.Equal(t, "test-group", parsed.Group)
	assert.Equal(t, uint32(42), parsed.Features)
	assert.Equal(t, "2.0", parsed.Version)
}

func TestParseHandshakePacket_WrongType(t *testing.T) {
	dataPkt := &TunnelPacket{
		Header: Header{Type: PacketTypeData},
		Data:   []byte("hello"),
	}
	_, err := ParseHandshakePacket(dataPkt)
	assert.Error(t, err)
}

func TestParseHandshakePacket_InvalidData(t *testing.T) {
	tunnelPkt := &TunnelPacket{
		Header: Header{Type: PacketTypeHandshake},
		Data:   []byte{0x01}, // too short
	}
	_, err := ParseHandshakePacket(tunnelPkt)
	assert.Error(t, err)
}

// === Coverage boost tests ===

func TestParseDataPacket_WrongType(t *testing.T) {
	pkt := &TunnelPacket{Header: Header{Type: PacketTypeClose}, Data: nil}
	_, err := ParseDataPacket(pkt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据包类型错误")
}

func TestParseClosePacket_WrongType(t *testing.T) {
	pkt := &TunnelPacket{Header: Header{Type: PacketTypeData}, Data: []byte("x")}
	_, err := ParseClosePacket(pkt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据包类型错误")
}

func TestGetData_Timeout(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s-timeout", mc)
	data, err := s.GetData()
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestGetData_FromBuffer(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s-getbuf", mc)
	s.PutData([]byte("hello"))
	data, err := s.GetData()
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)
}

func TestGetData_AfterClose(t *testing.T) {
	mc := newMockConn()
	s := NewTunnelStreamImpl("s-getclose", mc)
	s.Close()
	_, err := s.GetData()
	assert.Equal(t, ErrConnClosed, err)
}
