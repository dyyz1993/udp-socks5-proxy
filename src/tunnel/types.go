package tunnel

import (
	"errors"
)

// 这些定义已经在packet.go中定义，不需要重复定义
// PacketType 定义数据包的类型
// type PacketType uint8

// const (
// 	// PacketTypeHandshake 握手包类型
// 	PacketTypeHandshake PacketType = 1
// 	// PacketTypeData 数据包类型
// 	PacketTypeData PacketType = 2
// 	// PacketTypeHeartbeat 心跳包类型
// 	PacketTypeHeartbeat PacketType = 3
// 	// PacketTypeClose 关闭包类型
// 	PacketTypeClose PacketType = 4
// 	// PacketTypeError 错误包类型
// 	PacketTypeError PacketType = 5
// )

// ProtocolVersion 当前协议版本
// const ProtocolVersion uint8 = 1

// ConnectionState 连接状态类型
type ConnectionState int

// 连接状态定义
const (
	// StateInitialized 已初始化
	StateInitialized ConnectionState = iota
)

// 预定义的错误 - 部分错误已在packet.go中定义，这里仅保留不冲突的错误
var (
	// ErrStreamNotFound 流不存在
	ErrStreamNotFound = errors.New("stream not found")
	// ErrAuthFailed 认证失败
	ErrAuthFailed = errors.New("authentication failed")
	// ErrTargetUnreachable 目标不可达
	ErrTargetUnreachable = errors.New("target unreachable")
	// ErrTimeout 超时
	ErrTimeout = errors.New("operation timed out")
)

// Error codes
const (
	// ErrCodeInvalidPacket 无效的数据包
	ErrCodeInvalidPacket = 1001
	// ErrCodeStreamNotFound 找不到流
	ErrCodeStreamNotFound = 1005
	// ErrCodeAuthFailed 认证失败
	ErrCodeAuthFailed = 1002
	// ErrCodeTargetUnreachable 目标不可达
	ErrCodeTargetUnreachable = 1008
	// ErrCodeInternalError 内部错误
	ErrCodeInternalError = 1010
	// ErrCodeConnectionLimit 连接数限制
	ErrCodeConnectionLimit = 1003
	// ErrCodeVersionMismatch 版本不匹配
	ErrCodeVersionMismatch = 1004
	// ErrCodeConnectionError 连接错误
	ErrCodeConnectionError = 1006
	// ErrCodeConnectionClosed 连接已关闭
	ErrCodeConnectionClosed = 1007
	// ErrCodeTimeout 超时
	ErrCodeTimeout = 1009
	// ErrCodePacketHandlingError 数据包处理错误
	ErrCodePacketHandlingError = 1011
)
