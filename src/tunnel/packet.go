package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// 协议常量
const (
	ProtocolVersion = 1

	// 数据包类型
	PacketTypeHandshake  = 1
	PacketTypeData       = 2
	PacketTypeHeartbeat  = 3
	PacketTypeClose      = 4
	PacketTypeError      = 5
	PacketTypeFragmented = 6 // 新增：分片数据包类型

	// 错误码
	ErrorCodeGeneral     = 1
	ErrorCodeTimeout     = 2
	ErrorCodeUnsupported = 3

	// 连接状态
	StateInitial          = 0
	StateConnecting       = 1
	StateConnected        = 2
	StateDisconnecting    = 3
	StateClosed           = 4
	StateReconnecting     = 5
	StateReconnectWaiting = 6

	// 分片常量
	MaxUDPPacketSize    = 8192                                  // 安全的UDP包大小限制，比实际测出的9216小一些作为安全边界
	FragmentHeaderSize  = 20                                    // 预估分片头部大小(ConnectionID等信息)
	MaxFragmentDataSize = MaxUDPPacketSize - FragmentHeaderSize // 每个分片的最大数据大小
)

// 分片标记
const (
	FragmentFlagStart = 1 << 0 // 首个分片
	FragmentFlagEnd   = 1 << 1 // 最后分片
	FragmentFlagMore  = 1 << 2 // 有更多分片
)

// PacketType 数据包类型
type PacketType byte

// 错误类型
var (
	ErrConnClosed    = fmt.Errorf("connection closed")
	ErrInvalidPacket = fmt.Errorf("invalid packet")
	ErrEOF           = fmt.Errorf("EOF")
)

// Header 数据包头部
type Header struct {
	Version      uint8      // 协议版本
	Type         PacketType // 数据包类型
	Flags        uint8      // 标志位
	ConnectionID string     // 连接ID
	StreamID     string     // 数据流ID，仅数据和关闭包使用
}

// TunnelPacket 基础数据包结构
type TunnelPacket struct {
	Header Header
	Data   []byte
}

// Type 实现Packet接口
func (p *TunnelPacket) Type() PacketType {
	return p.Header.Type
}

// Bytes 将数据包转换为字节数组
func (tp *TunnelPacket) Bytes() []byte {
	// 打印Header信息(十进制)
	fmt.Printf("TunnelPacket Header: Version=%d, Type=%d, Flags=%d, ConnectionID=%s, StreamID=%s\n",
		tp.Header.Version, tp.Header.Type, tp.Header.Flags, tp.Header.ConnectionID, tp.Header.StreamID)

	// 创建缓冲区
	buf := bytes.NewBuffer(nil)

	// 写入版本
	buf.WriteByte(tp.Header.Version)

	// 写入类型
	buf.WriteByte(byte(tp.Header.Type))

	// 写入标志位
	buf.WriteByte(tp.Header.Flags)

	// 写入连接ID长度和ID
	idBytes := []byte(tp.Header.ConnectionID)
	binary.Write(buf, binary.BigEndian, uint16(len(idBytes)))
	buf.Write(idBytes)

	// 如果是数据包、关闭包或分片包，写入StreamID
	if tp.Header.Type == PacketTypeData || tp.Header.Type == PacketTypeClose || tp.Header.Type == PacketTypeFragmented {
		// 写入StreamID长度和StreamID
		streamIDBytes := []byte(tp.Header.StreamID)
		binary.Write(buf, binary.BigEndian, uint16(len(streamIDBytes)))
		buf.Write(streamIDBytes)
	}

	// 如果有数据，写入数据
	if len(tp.Data) > 0 {
		buf.Write(tp.Data)
	}

	return buf.Bytes()
}

// ParsePacket 从字节数据解析数据包
func ParsePacket(data []byte) (*TunnelPacket, error) {
	if len(data) < 5 { // 最小包长度: 版本(1) + 类型(1) + 标志位(1) + ID长度(2)
		return nil, ErrInvalidPacket
	}

	reader := bytes.NewReader(data)

	// 读取头部
	version, _ := reader.ReadByte()
	packetType, _ := reader.ReadByte()
	flags, _ := reader.ReadByte()

	// 读取连接ID
	var idLen uint16
	binary.Read(reader, binary.BigEndian, &idLen)
	if idLen > 36 || uint16(reader.Len()) < idLen { // UUID最长36字节
		return nil, ErrInvalidPacket
	}

	idBytes := make([]byte, idLen)
	reader.Read(idBytes)
	connectionID := string(idBytes)

	// 创建Header
	header := Header{
		Version:      version,
		Type:         PacketType(packetType),
		Flags:        flags,
		ConnectionID: connectionID,
	}

	// 如果是数据包、关闭包或分片包，读取StreamID
	if header.Type == PacketTypeData || header.Type == PacketTypeClose || header.Type == PacketTypeFragmented {
		// 读取StreamID长度
		var streamIDLen uint16
		binary.Read(reader, binary.BigEndian, &streamIDLen)
		if streamIDLen > 36 || uint16(reader.Len()) < streamIDLen { // UUID最长36字节
			return nil, ErrInvalidPacket
		}

		// 读取StreamID
		streamIDBytes := make([]byte, streamIDLen)
		reader.Read(streamIDBytes)
		header.StreamID = string(streamIDBytes)
	}

	// 读取剩余数据
	payload := make([]byte, reader.Len())
	reader.Read(payload)

	packet := &TunnelPacket{
		Header: header,
		Data:   payload,
	}

	return packet, nil
}

// HandshakePacket 握手包
type HandshakePacket struct {
	TunnelPacket
	Key      [32]byte // 握手密钥
	Group    string   // 分组名称
	Features uint32   // 特性标志
	Version  string   // 客户端版本
}

// NewHandshakePacket 创建新的握手包
func NewHandshakePacket(connectionID string, key [32]byte, group string, features uint32, version string) *HandshakePacket {
	// 序列化包体
	buf := &bytes.Buffer{}

	// 写入Key
	buf.Write(key[:])

	// 写入Group
	groupBytes := []byte(group)
	binary.Write(buf, binary.BigEndian, uint16(len(groupBytes)))
	buf.Write(groupBytes)

	// 写入Features
	binary.Write(buf, binary.BigEndian, features)

	// 写入Version
	versionBytes := []byte(version)
	binary.Write(buf, binary.BigEndian, uint16(len(versionBytes)))
	buf.Write(versionBytes)

	return &HandshakePacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeHandshake,
				Flags:        0,
				ConnectionID: connectionID,
			},
			Data: buf.Bytes(),
		},
		Key:      key,
		Group:    group,
		Features: features,
		Version:  version,
	}
}

// DataPacket 数据包
type DataPacket struct {
	TunnelPacket
	// StreamID字段已移至Header中
}

// NewDataPacket 创建新的数据包
func NewDataPacket(connectionID string, streamID string, data []byte) *DataPacket {
	// 直接使用业务数据创建数据包
	return &DataPacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeData,
				Flags:        0,
				ConnectionID: connectionID,
				StreamID:     streamID,
			},
			Data: data, // 直接使用原始业务数据，不再包含StreamID
		},
	}
}

// ParseDataPacket 从基础数据包解析数据包
func ParseDataPacket(packet *TunnelPacket) (*DataPacket, error) {
	if packet.Header.Type != PacketTypeData {
		return nil, fmt.Errorf("数据包类型错误: %d != %d", packet.Header.Type, PacketTypeData)
	}

	// StreamID已经在Header中，不需要再从数据中提取
	// 直接创建DataPacket
	return &DataPacket{
		TunnelPacket: *packet,
	}, nil
}

// Bytes 将数据包转换为字节数组
func (dp *DataPacket) Bytes() []byte {
	// 打印Header信息(十进制)
	fmt.Printf("DataPacket Header: Version=%d, Type=%d, Flags=%d, ConnectionID=%s, StreamID=%s\n",
		dp.TunnelPacket.Header.Version, dp.TunnelPacket.Header.Type,
		dp.TunnelPacket.Header.Flags, dp.TunnelPacket.Header.ConnectionID, dp.TunnelPacket.Header.StreamID)

	// 使用TunnelPacket的Bytes方法，它已经能处理StreamID
	return dp.TunnelPacket.Bytes()
}

// HeartbeatPacket 心跳包
type HeartbeatPacket struct {
	TunnelPacket
	Timestamp int64   // 时间戳
	Sequence  uint32  // 序列号
	Load      float32 // 负载信息
}

// NewHeartbeatPacket 创建新的心跳包
func NewHeartbeatPacket(connectionID string, sequence uint32, load float32) *HeartbeatPacket {
	// 序列化包体
	buf := &bytes.Buffer{}

	// 写入时间戳
	timestamp := time.Now().UnixNano()
	binary.Write(buf, binary.BigEndian, timestamp)

	// 写入序列号
	binary.Write(buf, binary.BigEndian, sequence)

	// 写入负载
	binary.Write(buf, binary.BigEndian, load)

	return &HeartbeatPacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeHeartbeat,
				Flags:        0,
				ConnectionID: connectionID,
			},
			Data: buf.Bytes(),
		},
		Timestamp: timestamp,
		Sequence:  sequence,
		Load:      load,
	}
}

// ClosePacket 关闭包
type ClosePacket struct {
	TunnelPacket
	// StreamID字段已移至Header中
}

// NewClosePacket 创建新的关闭包
func NewClosePacket(connectionID string, streamID string) *ClosePacket {
	return &ClosePacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeClose,
				Flags:        0,
				ConnectionID: connectionID,
				StreamID:     streamID,
			},
			Data: nil, // 关闭包不需要数据
		},
	}
}

// ParseClosePacket 从基础数据包解析关闭包
func ParseClosePacket(packet *TunnelPacket) (*ClosePacket, error) {
	if packet.Header.Type != PacketTypeClose {
		return nil, fmt.Errorf("数据包类型错误: %d != %d", packet.Header.Type, PacketTypeClose)
	}

	// StreamID已经在Header中，不需要再从数据中提取
	// 直接创建ClosePacket
	return &ClosePacket{
		TunnelPacket: *packet,
	}, nil
}

// Bytes 将数据包转换为字节数组
func (cp *ClosePacket) Bytes() []byte {
	// 打印Header信息(十进制)
	fmt.Printf("ClosePacket Header: Version=%d, Type=%d, Flags=%d, ConnectionID=%s, StreamID=%s\n",
		cp.TunnelPacket.Header.Version, cp.TunnelPacket.Header.Type,
		cp.TunnelPacket.Header.Flags, cp.TunnelPacket.Header.ConnectionID, cp.TunnelPacket.Header.StreamID)

	// 使用TunnelPacket的Bytes方法，它已经能处理StreamID
	return cp.TunnelPacket.Bytes()
}

// ErrorPacket 错误包
type ErrorPacket struct {
	TunnelPacket
	Code      int    // 错误码
	Message   string // 错误信息
	RelatedID string // 相关ID（连接ID或流ID）
}

// NewErrorPacket 创建新的错误包
func NewErrorPacket(connectionID string, code int, message string, relatedID string) *ErrorPacket {
	// 序列化包体
	buf := &bytes.Buffer{}

	// 写入错误码
	binary.Write(buf, binary.BigEndian, int32(code))

	// 写入错误信息
	msgBytes := []byte(message)
	binary.Write(buf, binary.BigEndian, uint16(len(msgBytes)))
	buf.Write(msgBytes)

	// 写入相关ID
	relatedIDBytes := []byte(relatedID)
	binary.Write(buf, binary.BigEndian, uint16(len(relatedIDBytes)))
	buf.Write(relatedIDBytes)

	return &ErrorPacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeError,
				Flags:        0,
				ConnectionID: connectionID,
			},
			Data: buf.Bytes(),
		},
		Code:      code,
		Message:   message,
		RelatedID: relatedID,
	}
}

// GenerateConnectionID 生成新的连接ID
func GenerateConnectionID() string {
	return uuid.New().String()
}

// GenerateStreamID 生成新的流ID
func GenerateStreamID() string {
	return uuid.New().String()
}

// ParseErrorPacket 从基础数据包解析错误包
func ParseErrorPacket(packet *TunnelPacket) (*ErrorPacket, error) {
	if packet.Header.Type != PacketTypeError {
		return nil, fmt.Errorf("数据包类型错误: %d != %d", packet.Header.Type, PacketTypeError)
	}

	// 检查数据长度
	if len(packet.Data) < 6 { // 错误码(4) + 消息长度字段(2) 至少需要6字节
		return nil, ErrInvalidPacket
	}

	reader := bytes.NewReader(packet.Data)

	// 读取错误码
	var errCode int32
	binary.Read(reader, binary.BigEndian, &errCode)

	// 读取错误消息
	var msgLen uint16
	binary.Read(reader, binary.BigEndian, &msgLen)

	if uint16(reader.Len()) < msgLen {
		return nil, ErrInvalidPacket
	}

	msgBytes := make([]byte, msgLen)
	reader.Read(msgBytes)
	message := string(msgBytes)

	// 读取相关ID
	var relatedIDLen uint16
	binary.Read(reader, binary.BigEndian, &relatedIDLen)

	if uint16(reader.Len()) < relatedIDLen {
		return nil, ErrInvalidPacket
	}

	relatedIDBytes := make([]byte, relatedIDLen)
	reader.Read(relatedIDBytes)
	relatedID := string(relatedIDBytes)

	return &ErrorPacket{
		TunnelPacket: *packet,
		Code:         int(errCode),
		Message:      message,
		RelatedID:    relatedID,
	}, nil
}

// Bytes 将数据包转换为字节数组
func (hp *HeartbeatPacket) Bytes() []byte {
	// 打印Header信息(十进制)
	fmt.Printf("HeartbeatPacket Header: Version=%d, Type=%d, Flags=%d, ConnectionID=%s\n",
		hp.TunnelPacket.Header.Version, hp.TunnelPacket.Header.Type,
		hp.TunnelPacket.Header.Flags, hp.TunnelPacket.Header.ConnectionID)

	// 创建缓冲区
	buf := bytes.NewBuffer(nil)

	// 写入版本
	buf.WriteByte(hp.TunnelPacket.Header.Version)

	// 写入类型
	buf.WriteByte(byte(hp.TunnelPacket.Header.Type))

	// 写入标志位
	buf.WriteByte(hp.TunnelPacket.Header.Flags)

	// 写入连接ID长度和ID
	idBytes := []byte(hp.TunnelPacket.Header.ConnectionID)
	binary.Write(buf, binary.BigEndian, uint16(len(idBytes)))
	buf.Write(idBytes)

	// 如果有数据，写入数据
	if len(hp.TunnelPacket.Data) > 0 {
		buf.Write(hp.TunnelPacket.Data)
	}

	return buf.Bytes()
}

// ParseHandshakePacket 从基础数据包解析握手包
func ParseHandshakePacket(packet *TunnelPacket) (*HandshakePacket, error) {
	if packet.Header.Type != PacketTypeHandshake {
		return nil, fmt.Errorf("数据包类型错误: %d != %d", packet.Header.Type, PacketTypeHandshake)
	}

	reader := bytes.NewReader(packet.Data)

	// 读取Key
	key := [32]byte{}
	if reader.Len() < 32 {
		return nil, ErrInvalidPacket
	}
	reader.Read(key[:])

	// 读取Group
	var groupLen uint16
	if err := binary.Read(reader, binary.BigEndian, &groupLen); err != nil {
		return nil, err
	}
	if uint16(reader.Len()) < groupLen {
		return nil, ErrInvalidPacket
	}
	groupBytes := make([]byte, groupLen)
	reader.Read(groupBytes)
	group := string(groupBytes)

	// 读取Features
	var features uint32
	if err := binary.Read(reader, binary.BigEndian, &features); err != nil {
		return nil, err
	}

	// 读取Version
	var versionLen uint16
	if err := binary.Read(reader, binary.BigEndian, &versionLen); err != nil {
		return nil, err
	}
	if uint16(reader.Len()) < versionLen {
		return nil, ErrInvalidPacket
	}
	versionBytes := make([]byte, versionLen)
	reader.Read(versionBytes)
	version := string(versionBytes)

	handshakePacket := &HandshakePacket{
		TunnelPacket: *packet,
		Key:          key,
		Group:        group,
		Features:     features,
		Version:      version,
	}

	return handshakePacket, nil
}

// Bytes 将握手包转换为字节数组
func (hp *HandshakePacket) Bytes() []byte {
	// 打印握手包信息
	fmt.Printf("HandshakePacket Header: Version=%d, Type=%d, Flags=%d, ConnectionID=%s\n",
		hp.TunnelPacket.Header.Version, hp.TunnelPacket.Header.Type,
		hp.TunnelPacket.Header.Flags, hp.TunnelPacket.Header.ConnectionID)

	// 使用TunnelPacket的Bytes方法
	return hp.TunnelPacket.Bytes()
}

// FragmentPacket 分片数据包
type FragmentPacket struct {
	TunnelPacket
	SequenceID     uint32     // 分片序列号
	TotalFragments uint32     // 总分片数
	FragmentIndex  uint32     // 当前分片索引
	Flags          uint8      // 分片标记
	OriginalType   PacketType // 原始数据包类型
}

// NewFragmentPacket 创建一个新的分片数据包
func NewFragmentPacket(connectionID string, streamID string, originalType PacketType, sequenceID uint32, totalFragments uint32, fragmentIndex uint32, flags uint8, fragmentData []byte) *FragmentPacket {
	// 为元数据创建缓冲区
	metaBuffer := bytes.Buffer{}

	// 写入原始包类型
	metaBuffer.WriteByte(byte(originalType))

	// 写入序列ID
	binary.Write(&metaBuffer, binary.BigEndian, sequenceID)

	// 写入总分片数
	binary.Write(&metaBuffer, binary.BigEndian, totalFragments)

	// 写入分片索引
	binary.Write(&metaBuffer, binary.BigEndian, fragmentIndex)

	// 写入标记
	metaBuffer.WriteByte(flags)

	// 附加分片数据
	metaBuffer.Write(fragmentData)

	return &FragmentPacket{
		TunnelPacket: TunnelPacket{
			Header: Header{
				Version:      ProtocolVersion,
				Type:         PacketTypeFragmented,
				Flags:        0,
				ConnectionID: connectionID,
				StreamID:     streamID, // 确保StreamID被正确设置
			},
			Data: metaBuffer.Bytes(),
		},
		SequenceID:     sequenceID,
		TotalFragments: totalFragments,
		FragmentIndex:  fragmentIndex,
		Flags:          flags,
		OriginalType:   originalType,
	}
}

// ParseFragmentPacket 解析分片数据包
func ParseFragmentPacket(packet *TunnelPacket) (*FragmentPacket, error) {
	if packet.Header.Type != PacketTypeFragmented {
		return nil, fmt.Errorf("数据包类型错误: %d != %d", packet.Header.Type, PacketTypeFragmented)
	}

	if len(packet.Data) < 14 { // 1(type) + 4(seqID) + 4(total) + 4(index) + 1(flags)
		return nil, fmt.Errorf("分片数据包太短")
	}

	buf := bytes.NewBuffer(packet.Data)

	// 读取原始类型
	originalType := PacketType(buf.Next(1)[0])

	// 读取序列ID
	var sequenceID uint32
	binary.Read(bytes.NewReader(buf.Next(4)), binary.BigEndian, &sequenceID)

	// 读取总分片数
	var totalFragments uint32
	binary.Read(bytes.NewReader(buf.Next(4)), binary.BigEndian, &totalFragments)

	// 读取分片索引
	var fragmentIndex uint32
	binary.Read(bytes.NewReader(buf.Next(4)), binary.BigEndian, &fragmentIndex)

	// 读取标记
	flags := buf.Next(1)[0]

	// 创建并返回FragmentPacket，不需要单独存储data变量
	return &FragmentPacket{
		TunnelPacket:   *packet,
		SequenceID:     sequenceID,
		TotalFragments: totalFragments,
		FragmentIndex:  fragmentIndex,
		Flags:          flags,
		OriginalType:   originalType,
	}, nil
}

// GetFragmentData 获取分片中的实际数据
func (fp *FragmentPacket) GetFragmentData() []byte {
	// 跳过元数据头(1+4+4+4+1=14字节)
	return fp.Data[14:]
}

// SplitPacket 将大型数据包分片
func SplitPacket(packet *TunnelPacket) []*FragmentPacket {
	if len(packet.Bytes()) <= MaxUDPPacketSize {
		// 如果数据包小于限制大小，不需要分片
		return nil
	}

	// 生成分片序列ID
	sequenceID := uint32(time.Now().UnixNano() & 0xFFFFFFFF)

	// 计算需要的分片数量
	totalData := len(packet.Data)
	totalFragments := (totalData + MaxFragmentDataSize - 1) / MaxFragmentDataSize

	// 创建分片列表
	fragments := make([]*FragmentPacket, 0, totalFragments)

	// 分割数据
	for i := 0; i < totalFragments; i++ {
		// 计算分片数据的范围
		start := i * MaxFragmentDataSize
		end := start + MaxFragmentDataSize
		if end > totalData {
			end = totalData
		}

		// 提取片段
		fragmentData := packet.Data[start:end]

		// 设置标记
		var flags uint8 = 0
		if i == 0 {
			flags |= FragmentFlagStart
		}
		if i == totalFragments-1 {
			flags |= FragmentFlagEnd
		}
		if i < totalFragments-1 {
			flags |= FragmentFlagMore
		}

		// 创建分片
		fragment := NewFragmentPacket(
			packet.Header.ConnectionID,
			packet.Header.StreamID, // 传递StreamID
			packet.Header.Type,
			sequenceID,
			uint32(totalFragments),
			uint32(i),
			flags,
			fragmentData,
		)

		fragments = append(fragments, fragment)
	}

	return fragments
}

// MergeFragments 将分片合并为原始数据包
func MergeFragments(fragments []*FragmentPacket) (*TunnelPacket, error) {
	if len(fragments) == 0 {
		return nil, errors.New("没有可合并的分片")
	}

	// 验证所有分片具有相同的连接ID和序列ID
	connectionID := fragments[0].Header.ConnectionID
	streamID := fragments[0].Header.StreamID // 获取StreamID
	sequenceID := fragments[0].SequenceID
	totalFragments := fragments[0].TotalFragments
	originalType := fragments[0].OriginalType

	// 验证分片数量
	if uint32(len(fragments)) != totalFragments {
		return nil, fmt.Errorf("分片数量不匹配: 期望 %d, 实际 %d", totalFragments, len(fragments))
	}

	// 检查所有分片
	for i, fragment := range fragments {
		if fragment.Header.ConnectionID != connectionID {
			return nil, fmt.Errorf("分片 %d 的连接ID不匹配", i)
		}
		if fragment.Header.StreamID != streamID {
			return nil, fmt.Errorf("分片 %d 的流ID不匹配", i)
		}
		if fragment.SequenceID != sequenceID {
			return nil, fmt.Errorf("分片 %d 的序列ID不匹配", i)
		}
		if fragment.TotalFragments != totalFragments {
			return nil, fmt.Errorf("分片 %d 的总分片数不匹配", i)
		}
		if fragment.OriginalType != originalType {
			return nil, fmt.Errorf("分片 %d 的原始类型不匹配", i)
		}
		if uint32(i) != fragment.FragmentIndex {
			return nil, fmt.Errorf("分片索引不连续, 期望 %d, 实际 %d", i, fragment.FragmentIndex)
		}
	}

	// 创建数据缓冲区
	var mergedData bytes.Buffer

	// 按顺序合并数据
	for _, fragment := range fragments {
		fragmentData := fragment.GetFragmentData()
		mergedData.Write(fragmentData)
	}

	// 创建合并后的数据包
	mergedPacket := &TunnelPacket{
		Header: Header{
			Version:      ProtocolVersion,
			Type:         originalType,
			Flags:        0,
			ConnectionID: connectionID,
			StreamID:     streamID, // 确保使用正确的StreamID
		},
		Data: mergedData.Bytes(),
	}

	return mergedPacket, nil
}
