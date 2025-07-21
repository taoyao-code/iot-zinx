package protocol

import (
	"encoding/hex"
	"testing"
)

// TestUnifiedDNYBuilder_BuildDNYPacket 测试统一DNY构建器的数据包构建功能
func TestUnifiedDNYBuilder_BuildDNYPacket(t *testing.T) {
	builder := NewUnifiedDNYBuilder()

	// 测试用例：构建一个简单的DNY数据包
	physicalID := uint32(0x04A228CD)
	messageID := uint16(0x1234)
	command := uint8(0x82)
	data := []byte{0x01, 0x02, 0x03}

	packet := builder.BuildDNYPacket(physicalID, messageID, command, data)

	// 验证包头
	if string(packet[:3]) != "DNY" {
		t.Errorf("包头错误：期望'DNY'，实际'%s'", string(packet[:3]))
	}

	// 验证长度字段（小端序）
	expectedContentLen := 4 + 2 + 1 + len(data) + 2 // PhysicalID + MessageID + Command + Data + Checksum
	actualContentLen := int(packet[3]) | (int(packet[4]) << 8)
	if actualContentLen != expectedContentLen {
		t.Errorf("长度字段错误：期望%d，实际%d", expectedContentLen, actualContentLen)
	}

	// 验证物理ID（小端序）
	actualPhysicalID := uint32(packet[5]) | (uint32(packet[6]) << 8) | (uint32(packet[7]) << 16) | (uint32(packet[8]) << 24)
	if actualPhysicalID != physicalID {
		t.Errorf("物理ID错误：期望0x%08X，实际0x%08X", physicalID, actualPhysicalID)
	}

	// 验证消息ID（小端序）
	actualMessageID := uint16(packet[9]) | (uint16(packet[10]) << 8)
	if actualMessageID != messageID {
		t.Errorf("消息ID错误：期望0x%04X，实际0x%04X", messageID, actualMessageID)
	}

	// 验证命令
	if packet[11] != command {
		t.Errorf("命令错误：期望0x%02X，实际0x%02X", command, packet[11])
	}

	// 验证数据
	for i, b := range data {
		if packet[12+i] != b {
			t.Errorf("数据[%d]错误：期望0x%02X，实际0x%02X", i, b, packet[12+i])
		}
	}

	// 验证校验和
	checksumPos := len(packet) - 2
	expectedChecksum := builder.CalculateChecksum(packet[:checksumPos])
	actualChecksum := uint16(packet[checksumPos]) | (uint16(packet[checksumPos+1]) << 8)
	if actualChecksum != expectedChecksum {
		t.Errorf("校验和错误：期望0x%04X，实际0x%04X", expectedChecksum, actualChecksum)
	}

	t.Logf("构建的数据包: %s", hex.EncodeToString(packet))
}

// TestUnifiedDNYBuilder_ValidatePacket 测试数据包验证功能
func TestUnifiedDNYBuilder_ValidatePacket(t *testing.T) {
	builder := NewUnifiedDNYBuilder()

	// 构建一个有效的数据包
	packet := builder.BuildDNYPacket(0x04A228CD, 0x1234, 0x82, []byte{0x01, 0x02, 0x03})

	// 验证有效数据包
	err := builder.ValidatePacket(packet)
	if err != nil {
		t.Errorf("有效数据包验证失败：%v", err)
	}

	// 测试无效包头
	invalidPacket := make([]byte, len(packet))
	copy(invalidPacket, packet)
	invalidPacket[0] = 'X'
	err = builder.ValidatePacket(invalidPacket)
	if err == nil {
		t.Error("无效包头应该验证失败")
	}

	// 测试长度不匹配
	shortPacket := packet[:len(packet)-1]
	err = builder.ValidatePacket(shortPacket)
	if err == nil {
		t.Error("长度不匹配应该验证失败")
	}
}

// TestBuildUnifiedDNYPacket 测试全局便捷函数
func TestBuildUnifiedDNYPacket(t *testing.T) {
	physicalID := uint32(0x04A228CD)
	messageID := uint16(0x1234)
	command := uint8(0x82)
	data := []byte{0x01, 0x02, 0x03}

	packet := BuildUnifiedDNYPacket(physicalID, messageID, command, data)

	// 验证包头
	if string(packet[:3]) != "DNY" {
		t.Errorf("包头错误：期望'DNY'，实际'%s'", string(packet[:3]))
	}

	// 验证数据包可以通过验证
	err := ValidateUnifiedDNYPacket(packet)
	if err != nil {
		t.Errorf("数据包验证失败：%v", err)
	}

	t.Logf("全局函数构建的数据包: %s", hex.EncodeToString(packet))
}

// TestCompatibilityFunctions 测试兼容性函数
func TestCompatibilityFunctions(t *testing.T) {
	physicalID := uint32(0x04A228CD)
	messageID := uint16(0x1234)
	command := uint8(0x82)
	data := []byte{0x01, 0x02, 0x03}

	// 测试兼容性函数
	responsePacket := BuildDNYResponsePacket(physicalID, messageID, command, data)
	requestPacket := BuildDNYRequestPacket(physicalID, messageID, command, data)
	internalPacket := buildDNYPacket(physicalID, messageID, command, data)

	// 所有函数应该产生相同的结果
	if hex.EncodeToString(responsePacket) != hex.EncodeToString(requestPacket) {
		t.Error("响应包和请求包应该相同")
	}

	if hex.EncodeToString(responsePacket) != hex.EncodeToString(internalPacket) {
		t.Error("响应包和内部包应该相同")
	}

	t.Logf("兼容性函数测试通过，数据包: %s", hex.EncodeToString(responsePacket))
}

// BenchmarkUnifiedDNYBuilder 性能测试
func BenchmarkUnifiedDNYBuilder(b *testing.B) {
	builder := NewUnifiedDNYBuilder()
	physicalID := uint32(0x04A228CD)
	messageID := uint16(0x1234)
	command := uint8(0x82)
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packet := builder.BuildDNYPacket(physicalID, messageID, command, data)
		_ = packet
	}
}
