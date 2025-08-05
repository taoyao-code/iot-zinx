package tests

import (
	"encoding/hex"
	"testing"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// TestChecksumValidation 验证校验码修复效果
func TestChecksumValidation(t *testing.T) {
	// 测试用例：用户提供的协议包数据
	testCases := []struct {
		name             string
		rawPacket        string // 原始协议包（十六进制字符串）
		expectedChecksum string // 期望的校验码（十六进制字符串，小端序）
		description      string
	}{
		{
			name:             "设备定位命令校验",
			rawPacket:        "444E590A00F36CA204A0C096010604",
			expectedChecksum: "f104", // 0x04F1的小端序表示（小写）
			description:      "设备定位命令(0x96)的校验码应该正确计算",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 解析原始协议包
			rawData, err := hex.DecodeString(tc.rawPacket)
			if err != nil {
				t.Fatalf("解析原始协议包失败: %v", err)
			}

			// 提取校验范围的数据（从"DNY"头开始到校验码前的所有字节）
			if len(rawData) < 4 {
				t.Fatalf("协议包长度不足: %d", len(rawData))
			}

			// 校验范围：除了最后2字节的校验码
			checksumData := rawData[:len(rawData)-2]

			// 使用修复后的校验函数计算校验码
			calculatedChecksum := dny_protocol.CalculateDNYChecksum(checksumData)

			// 将计算结果转换为小端序十六进制字符串
			checksumBytes := []byte{byte(calculatedChecksum), byte(calculatedChecksum >> 8)}
			calculatedHex := hex.EncodeToString(checksumBytes)

			// 验证计算结果
			expectedHex := tc.expectedChecksum
			if calculatedHex != expectedHex {
				t.Errorf("校验码计算错误:\n期望: %s\n实际: %s\n描述: %s",
					expectedHex, calculatedHex, tc.description)

				// 输出详细的调试信息
				t.Logf("原始协议包: %s", tc.rawPacket)
				t.Logf("校验范围数据: %s", hex.EncodeToString(checksumData))
				t.Logf("计算的校验码值: 0x%04X", calculatedChecksum)
				t.Logf("小端序表示: %s", calculatedHex)
			} else {
				t.Logf("✅ 校验码计算正确: %s", calculatedHex)
			}
		})
	}
}

// TestBuildDNYPacketChecksum 测试协议包构建的校验码
func TestBuildDNYPacketChecksum(t *testing.T) {
	// 测试构建设备定位协议包
	physicalID := uint32(0x04A26CF3) // 10644723
	messageID := uint16(0xC0A0)      // 小端序
	command := uint8(0x96)           // 设备定位命令
	data := []byte{0x01}             // 定位时间1秒

	// 构建协议包
	packet := dny_protocol.BuildDNYPacket(physicalID, messageID, command, data)

	// 验证协议包长度
	expectedLength := 3 + 2 + 4 + 2 + 1 + 1 + 2 // DNY + 长度 + 物理ID + 消息ID + 命令 + 数据 + 校验码
	if len(packet) != expectedLength {
		t.Errorf("协议包长度错误: 期望 %d, 实际 %d", expectedLength, len(packet))
	}

	// 提取校验码（最后2字节，小端序）
	if len(packet) < 2 {
		t.Fatalf("协议包长度不足，无法提取校验码")
	}

	actualChecksumBytes := packet[len(packet)-2:]
	actualChecksum := uint16(actualChecksumBytes[0]) | (uint16(actualChecksumBytes[1]) << 8)

	// 手动计算期望的校验码
	checksumData := packet[:len(packet)-2]
	expectedChecksum := dny_protocol.CalculateDNYChecksum(checksumData)

	if actualChecksum != expectedChecksum {
		t.Errorf("构建的协议包校验码错误:\n期望: 0x%04X\n实际: 0x%04X",
			expectedChecksum, actualChecksum)
		t.Logf("协议包: %s", hex.EncodeToString(packet))
		t.Logf("校验范围: %s", hex.EncodeToString(checksumData))
	} else {
		t.Logf("✅ 协议包构建校验码正确: 0x%04X", actualChecksum)
	}
}

// TestManualChecksumCalculation 手动验证校验码计算
func TestManualChecksumCalculation(t *testing.T) {
	// 用户提供的测试数据
	testData := "444E590A00F36CA204A0C09601" // 校验范围的数据

	data, err := hex.DecodeString(testData)
	if err != nil {
		t.Fatalf("解析测试数据失败: %v", err)
	}

	// 手动计算累加和
	var manualSum uint16
	for i, b := range data {
		manualSum += uint16(b)
		t.Logf("字节 %d: 0x%02X, 累计和: 0x%04X", i, b, manualSum)
	}

	// 使用函数计算
	functionSum := dny_protocol.CalculateDNYChecksum(data)

	// 验证结果一致性
	if manualSum != functionSum {
		t.Errorf("校验码计算不一致:\n手动计算: 0x%04X\n函数计算: 0x%04X",
			manualSum, functionSum)
	} else {
		t.Logf("✅ 校验码计算一致: 0x%04X", manualSum)

		// 转换为小端序表示
		checksumBytes := []byte{byte(manualSum), byte(manualSum >> 8)}
		t.Logf("✅ 小端序表示: %s", hex.EncodeToString(checksumBytes))
	}
}
