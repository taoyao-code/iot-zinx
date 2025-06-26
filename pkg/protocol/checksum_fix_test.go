package protocol

import (
	"encoding/hex"
	"testing"
)

// TestChecksumCalculation_UserReportedBug 测试用户报告的校验和计算错误
// 新报文: 444e590a00f36ca2040600960aa003 (命令0x96设备定位)
// 校验和: A003 (正确)
func TestChecksumCalculation_UserReportedBug(t *testing.T) {
	// 用户提供的报文（命令0x96设备定位）
	rawHex := "444e590a00f36ca2040600960aa003"
	data, err := hex.DecodeString(rawHex)
	if err != nil {
		t.Fatalf("十六进制解码失败: %v", err)
	}

	// 提取校验和计算范围（从包头到校验和前的所有字节）
	checksumData := data[0 : len(data)-2] // 排除最后2字节校验和

	// 使用修复后的校验和计算函数
	actualChecksum, err := CalculatePacketChecksumInternal(checksumData)
	if err != nil {
		t.Fatalf("校验和计算失败: %v", err)
	}

	// 根据新报文计算的正确校验和：03A0 (小端序: A0 03)
	expectedChecksum := uint16(0x03A0)

	if actualChecksum != expectedChecksum {
		t.Errorf("校验和计算错误:\n"+
			"  原始报文: %s\n"+
			"  计算范围: %02X (长度: %d字节)\n"+
			"  期望校验和: 0x%04X (小端序: %02X %02X)\n"+
			"  实际校验和: 0x%04X (小端序: %02X %02X)\n"+
			"  原报文校验: %02X %02X (错误)",
			rawHex,
			checksumData, len(checksumData),
			expectedChecksum, byte(expectedChecksum), byte(expectedChecksum>>8),
			actualChecksum, byte(actualChecksum), byte(actualChecksum>>8),
			data[len(data)-2], data[len(data)-1])
	}

	t.Logf("✅ 校验和计算修复验证成功:\n"+
		"  原始报文: %s\n"+
		"  计算范围: %02X\n"+
		"  正确校验和: 0x%04X (小端序: %02X %02X)",
		rawHex,
		checksumData,
		actualChecksum, byte(actualChecksum), byte(actualChecksum>>8))
}

// TestChecksumCalculation_Command22Response 测试命令22响应的校验和计算
func TestChecksumCalculation_Command22Response(t *testing.T) {
	// 模拟命令22的响应数据包构建
	physicalID := uint32(0x04A228CD) // CD28A204 (小端序)
	messageID := uint16(0x0879)      // 7908 (小端序)
	command := uint8(0x22)
	responseData := []byte{0x63, 0xEE, 0x5C, 0x68} // 4字节时间戳

	// 构建数据包（不包含校验和）
	packet := make([]byte, 0, 18)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度字段（小端序）- 物理ID(4) + 消息ID(2) + 命令(1) + 数据(4) + 校验(2) = 13
	contentLen := uint16(4 + 2 + 1 + len(responseData) + 2)
	packet = append(packet, byte(contentLen), byte(contentLen>>8))

	// 物理ID（小端序）
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端序）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 响应数据
	packet = append(packet, responseData...)

	// 计算校验和
	checksum, err := CalculatePacketChecksumInternal(packet)
	if err != nil {
		t.Fatalf("校验和计算失败: %v", err)
	}

	// 添加校验和
	packet = append(packet, byte(checksum), byte(checksum>>8))

	t.Logf("✅ 命令22响应数据包构建:\n"+
		"  物理ID: 0x%08X\n"+
		"  消息ID: 0x%04X\n"+
		"  命令: 0x%02X\n"+
		"  响应数据: %02X\n"+
		"  校验和: 0x%04X (小端序: %02X %02X)\n"+
		"  完整数据包: %s",
		physicalID, messageID, command,
		responseData,
		checksum, byte(checksum), byte(checksum>>8),
		hex.EncodeToString(packet))
}

// TestChecksumCalculation_MultipleFrames 测试多种DNY帧的校验和计算
func TestChecksumCalculation_MultipleFrames(t *testing.T) {
	testCases := []struct {
		name             string
		hexData          string // 不包含校验和的数据
		expectedChecksum uint16
		description      string
	}{
		{
			name:             "用户报告的命令22帧",
			hexData:          "444E590D00CD28A20479082263EE5C68",
			expectedChecksum: 0x054B,
			description:      "命令22获取服务器时间",
		},
		{
			name:             "标准心跳帧",
			hexData:          "444E590900F36CA204020012",
			expectedChecksum: 0x030D,
			description:      "设备心跳包",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Fatalf("十六进制解码失败: %v", err)
			}

			checksum, err := CalculatePacketChecksumInternal(data)
			if err != nil {
				t.Fatalf("校验和计算失败: %v", err)
			}

			if checksum != tc.expectedChecksum {
				t.Errorf("%s - 校验和计算错误:\n"+
					"  数据: %s\n"+
					"  期望: 0x%04X\n"+
					"  实际: 0x%04X",
					tc.description, tc.hexData,
					tc.expectedChecksum, checksum)
			} else {
				t.Logf("✅ %s - 校验和计算正确: 0x%04X", tc.description, checksum)
			}
		})
	}
}
