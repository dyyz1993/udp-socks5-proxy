package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

func TestFragmentCache_AddFragment(t *testing.T) {
	// 创建一个新的分片缓存
	fc := &fragmentCache{
		fragments:  make(map[uint32][]*tunnel.FragmentPacket),
		expireTime: make(map[uint32]time.Time),
	}

	// 创建测试分片
	fragment1 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 序列ID
		3, // 总分片数
		0, // 分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("fragment1"),
	)

	// 添加第一个分片
	packet, err := fc.addFragment(fragment1)
	assert.NoError(t, err, "添加第一个分片不应该返回错误")
	assert.Nil(t, packet, "添加第一个分片不应该返回完整包")

	// 验证分片被正确存储
	assert.Len(t, fc.fragments, 1, "应该有一个序列ID的分片列表")
	assert.Len(t, fc.fragments[1], 3, "分片列表长度应该是总分片数")
	assert.Equal(t, fragment1, fc.fragments[1][0], "分片应该在正确的位置")

	// 创建第二个分片
	fragment2 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		3, // 总分片数
		1, // 分片索引
		tunnel.FragmentFlagMore,
		[]byte("fragment2"),
	)

	// 添加第二个分片
	packet, err = fc.addFragment(fragment2)
	assert.NoError(t, err, "添加第二个分片不应该返回错误")
	assert.Nil(t, packet, "添加第二个分片不应该返回完整包")

	// 创建第三个分片
	fragment3 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		3, // 总分片数
		2, // 分片索引
		tunnel.FragmentFlagEnd,
		[]byte("fragment3"),
	)

	// 添加第三个分片，这时应该能合并成完整的包
	packet, err = fc.addFragment(fragment3)
	assert.NoError(t, err, "添加第三个分片不应该返回错误")
	assert.NotNil(t, packet, "添加最后一个分片应该返回完整包")

	// 验证合并后分片被清理
	assert.Len(t, fc.fragments, 0, "合并后应该清理分片")
	assert.Len(t, fc.expireTime, 0, "合并后应该清理过期时间")

	// 验证合并后的包
	assert.Equal(t, "test-conn", packet.Header.ConnectionID, "合并包连接ID不匹配")
	assert.Equal(t, "test-stream", packet.Header.StreamID, "合并包流ID不匹配")
	assert.Equal(t, tunnel.PacketTypeData, int(packet.Header.Type), "合并包类型不匹配")

	// 验证数据内容
	expectedData := []byte("fragment1fragment2fragment3")
	assert.Equal(t, expectedData, packet.Data, "合并包数据不匹配")
}

func TestFragmentCache_CleanExpired(t *testing.T) {
	// 创建一个新的分片缓存
	fc := &fragmentCache{
		fragments:  make(map[uint32][]*tunnel.FragmentPacket),
		expireTime: make(map[uint32]time.Time),
	}

	// 创建测试分片
	fragment1 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 序列ID
		2, // 总分片数
		0, // 分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("fragment1"),
	)

	// 添加第一个分片，但设置很短的过期时间
	fc.fragments[1] = make([]*tunnel.FragmentPacket, 2)
	fc.fragments[1][0] = fragment1
	fc.expireTime[1] = time.Now().Add(-1 * time.Second) // 已经过期

	// 创建第二个序列的分片，设置较长的过期时间
	fragment2 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		2, // 不同的序列ID
		2, // 总分片数
		0, // 分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("fragment2"),
	)

	fc.fragments[2] = make([]*tunnel.FragmentPacket, 2)
	fc.fragments[2][0] = fragment2
	fc.expireTime[2] = time.Now().Add(30 * time.Second) // 未过期

	// 调用清理过期分片
	fc.cleanExpired()

	// 验证过期的分片已被清理，未过期的保留
	assert.Len(t, fc.fragments, 1, "应该有一个未过期的序列")
	assert.Nil(t, fc.fragments[1], "过期的序列应该被删除")
	assert.NotNil(t, fc.fragments[2], "未过期的序列应该保留")
	assert.Len(t, fc.expireTime, 1, "应该有一个未过期的时间记录")
	assert.NotContains(t, fc.expireTime, uint32(1), "过期的时间记录应该被删除")
	assert.Contains(t, fc.expireTime, uint32(2), "未过期的时间记录应该保留")
}

func TestFragmentCache_OutOfOrderFragments(t *testing.T) {
	// 创建一个新的分片缓存
	fc := &fragmentCache{
		fragments:  make(map[uint32][]*tunnel.FragmentPacket),
		expireTime: make(map[uint32]time.Time),
	}

	// 创建测试分片，但以乱序添加
	fragment2 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 序列ID
		3, // 总分片数
		1, // 分片索引
		tunnel.FragmentFlagMore,
		[]byte("fragment2"),
	)

	// 先添加中间的分片
	packet, err := fc.addFragment(fragment2)
	assert.NoError(t, err, "添加中间分片不应该返回错误")
	assert.Nil(t, packet, "添加中间分片不应该返回完整包")

	// 验证分片被正确存储
	assert.Len(t, fc.fragments, 1, "应该有一个序列ID的分片列表")
	assert.Len(t, fc.fragments[1], 3, "分片列表长度应该是总分片数")
	assert.Equal(t, fragment2, fc.fragments[1][1], "中间分片应该在正确的位置")

	// 创建第一个分片
	fragment1 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		3, // 总分片数
		0, // 分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("fragment1"),
	)

	// 添加第一个分片
	packet, err = fc.addFragment(fragment1)
	assert.NoError(t, err, "添加第一个分片不应该返回错误")
	assert.Nil(t, packet, "添加第一个分片不应该返回完整包")

	// 验证第一个分片被正确存储
	assert.Equal(t, fragment1, fc.fragments[1][0], "第一个分片应该在正确的位置")

	// 创建第三个分片
	fragment3 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		3, // 总分片数
		2, // 分片索引
		tunnel.FragmentFlagEnd,
		[]byte("fragment3"),
	)

	// 添加第三个分片，这时应该能合并成完整的包
	packet, err = fc.addFragment(fragment3)
	assert.NoError(t, err, "添加第三个分片不应该返回错误")
	assert.NotNil(t, packet, "添加最后一个分片应该返回完整包")

	// 验证合并后的包数据
	expectedData := []byte("fragment1fragment2fragment3")
	assert.Equal(t, expectedData, packet.Data, "合并包数据不匹配")
}

func TestFragmentCache_DuplicateFragmentIndex(t *testing.T) {
	// 创建一个新的分片缓存
	fc := &fragmentCache{
		fragments:  make(map[uint32][]*tunnel.FragmentPacket),
		expireTime: make(map[uint32]time.Time),
	}

	// 创建第一个分片
	fragment1 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 序列ID
		2, // 总分片数
		0, // 分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("fragment1"),
	)

	// 添加第一个分片
	packet, err := fc.addFragment(fragment1)
	assert.NoError(t, err, "添加第一个分片不应该返回错误")
	assert.Nil(t, packet, "添加第一个分片不应该返回完整包")

	// 创建重复索引的分片
	duplicateFragment := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		2, // 总分片数
		0, // 相同的分片索引
		tunnel.FragmentFlagStart|tunnel.FragmentFlagMore,
		[]byte("duplicate"),
	)

	// 添加重复索引的分片，应该覆盖原来的分片
	packet, err = fc.addFragment(duplicateFragment)
	assert.NoError(t, err, "添加重复索引的分片不应该返回错误")
	assert.Nil(t, packet, "添加重复索引的分片不应该返回完整包")

	// 验证重复索引的分片已替换原分片
	assert.Equal(t, duplicateFragment, fc.fragments[1][0], "重复索引的分片应该替换原分片")

	// 创建第二个分片
	fragment2 := tunnel.NewFragmentPacket(
		"test-conn",
		"test-stream",
		tunnel.PacketType(tunnel.PacketTypeData),
		1, // 相同的序列ID
		2, // 总分片数
		1, // 分片索引
		tunnel.FragmentFlagEnd,
		[]byte("fragment2"),
	)

	// 添加第二个分片，这时应该能合并成完整的包
	packet, err = fc.addFragment(fragment2)
	assert.NoError(t, err, "添加第二个分片不应该返回错误")
	assert.NotNil(t, packet, "添加最后一个分片应该返回完整包")

	// 验证合并后的包数据，应该使用重复索引的分片数据
	expectedData := []byte("duplicatefragment2")
	assert.Equal(t, expectedData, packet.Data, "合并包数据不匹配")
}
