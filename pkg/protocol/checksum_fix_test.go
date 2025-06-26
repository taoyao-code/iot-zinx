package protocol

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestChecksumCalculation_UserReportedBug 测试用户报告的校验和计算错误
// 新报文: 444e590a00f36ca2040600960aa003
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

// TestChecksumCalculation_RealNetworkPackets 测试基于真实网络抓包数据的校验和计算
// 这些数据来自实际设备与服务器的通信记录，确保我们的校验和算法与真实设备兼容
func TestChecksumCalculation_RealNetworkPackets(t *testing.T) {
	testCases := []struct {
		name             string
		hexData          string // 不包含校验和的数据部分
		expectedChecksum uint16
		description      string
		fullPacket       string // 完整包数据（包含校验和）
	}{
		{
			name:             "设备定位包1",
			hexData:          "444e590a00cd28a204470096 0a",
			expectedChecksum: 0x0377,
			description:      "物理ID 0x04a228cd, 消息ID 0x0047, 命令0x96定位，数据0a",
			fullPacket:       "444e590a00cd28a204470096 0a 7703",
		},
		{
			name:             "设备定位包2",
			hexData:          "444e590a00f36ca204480096 0a",
			expectedChecksum: 0x03e2,
			description:      "物理ID 0x04a26cf3, 消息ID 0x0048, 命令0x96定位，数据0a",
			fullPacket:       "444e590a00f36ca204480096 0a e203",
		},
		{
			name:             "心跳包请求",
			hexData:          "444e590900f36ca204020012",
			expectedChecksum: 0x030d,
			description:      "物理ID 0x04a26cf3, 消息ID 0x0002, 命令0x12心跳",
			fullPacket:       "444e590900f36ca204020012 0d03",
		},
		{
			name:             "心跳包回复",
			hexData:          "444e590d00f36ca20402001291f75c68",
			expectedChecksum: 0x055d,
			description:      "服务器回复心跳，包含时间戳",
			fullPacket:       "444e590d00f36ca20402001291f75c68 5d05",
		},
		{
			name:             "设备信息上报包",
			hexData:          "444e592c00cd28a20449008200e8030000010105005445535f30344132323843445f3030000000000000000002000000",
			expectedChecksum: 0x0843,
			description:      "物理ID 0x04a228cd, 消息ID 0x0049, 命令0x82信息上报",
			fullPacket:       "444e592c00cd28a20449008200e8030000010105005445535f3034413232384344 5f3030000000000000000002000000 4308",
		},
		{
			name:             "充电控制命令1",
			hexData:          "444e590a00cd28a2046e00960a",
			expectedChecksum: 0x039e,
			description:      "物理ID 0x04a228cd, 消息ID 0x006e, 命令0x96定位，数据0a",
			fullPacket:       "444e590a00cd28a2046e0096 0a 9e03",
		},
		{
			name:             "充电控制命令2",
			hexData:          "444e590a00cd28a2046f00960a",
			expectedChecksum: 0x039f,
			description:      "物理ID 0x04a228cd, 消息ID 0x006f, 命令0x96定位，数据0a",
			fullPacket:       "444e590a00cd28a2046f0096 0a 9f03",
		},
		{
			name:             "获取服务器时间1",
			hexData:          "444e590900f36ca204030922",
			expectedChecksum: 0x0327,
			description:      "物理ID 0x04a26cf3, 消息ID 0x0903, 命令0x22获取时间",
			fullPacket:       "444e590900f36ca204030922 2703",
		},
		{
			name:             "获取服务器时间回复",
			hexData:          "444e590d00f36ca204030922cff75c68",
			expectedChecksum: 0x05b5,
			description:      "服务器回复时间，包含时间戳",
			fullPacket:       "444e590d00f36ca204030922cff75c68 b505",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 移除空格并解码数据
			cleanHexData := strings.ReplaceAll(tc.hexData, " ", "")
			data, err := hex.DecodeString(cleanHexData)
			if err != nil {
				t.Fatalf("十六进制解码失败: %v", err)
			}

			// 计算校验和
			checksum, err := CalculatePacketChecksumInternal(data)
			if err != nil {
				t.Fatalf("校验和计算失败: %v", err)
			}

			// 验证校验和
			if checksum != tc.expectedChecksum {
				t.Errorf("%s - 校验和计算错误:\n"+
					"  数据: %s\n"+
					"  期望: 0x%04X (小端序: %02X %02X)\n"+
					"  实际: 0x%04X (小端序: %02X %02X)\n"+
					"  完整包: %s",
					tc.description, cleanHexData,
					tc.expectedChecksum, byte(tc.expectedChecksum), byte(tc.expectedChecksum>>8),
					checksum, byte(checksum), byte(checksum>>8),
					tc.fullPacket)
			} else {
				t.Logf("✅ %s - 校验和计算正确: 0x%04X (小端序: %02X %02X)",
					tc.description, checksum, byte(checksum), byte(checksum>>8))
			}
		})
	}
}
