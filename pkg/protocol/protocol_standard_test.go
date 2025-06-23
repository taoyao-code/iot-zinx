package protocol

import (
	"encoding/hex"
	"testing"
)

// 🔒 永久固定的协议解析标准测试（无外部依赖）
// 这些测试用例基于真实设备数据，一旦通过，协议解析算法永久不变！

func TestICCIDValidation_Standard(t *testing.T) {
	testCases := []struct {
		name     string
		iccid    string
		expected bool
		reason   string
	}{
		{
			name:     "真实设备ICCID_包含字母D",
			iccid:    "898604D9162390488297",
			expected: true,
			reason:   "真实设备ICCID，包含十六进制字符D",
		},
		{
			name:     "标准中国移动ICCID_纯数字",
			iccid:    "89860429165872938875",
			expected: true,
			reason:   "标准20位数字ICCID",
		},
		{
			name:     "包含字母A的ICCID",
			iccid:    "898604A9162390488297",
			expected: true,
			reason:   "十六进制字符A",
		},
		{
			name:     "包含字母F的ICCID",
			iccid:    "898604F9162390488297",
			expected: true,
			reason:   "十六进制字符F",
		},
		{
			name:     "包含小写字母的ICCID",
			iccid:    "898604d9162390488297",
			expected: true,
			reason:   "小写十六进制字符也应支持",
		},
		{
			name:     "非法字符G",
			iccid:    "898604G9162390488297",
			expected: false,
			reason:   "G不是十六进制字符",
		},
		{
			name:     "长度不足19位",
			iccid:    "8986042916239048829",
			expected: false,
			reason:   "长度不足20位",
		},
		{
			name:     "长度超过21位",
			iccid:    "898604291623904882977",
			expected: false,
			reason:   "长度超过20位",
		},
		{
			name:     "不以89开头",
			iccid:    "788604D9162390488297",
			expected: false,
			reason:   "不符合ITU-T E.118标准前缀",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.iccid)
			result := isValidICCIDStrict(data)
			if result != tc.expected {
				t.Errorf("ICCID验证失败: %s - %s, 期望: %v, 得到: %v",
					tc.iccid, tc.reason, tc.expected, result)
			}
		})
	}
}

func TestDNYProtocolParsing_Standard(t *testing.T) {
	testCases := []struct {
		name               string
		hexData            string
		expectedValid      bool
		expectedPhysicalID uint32
		expectedCommand    uint8
		expectedMessageID  uint16
		expectedChecksum   uint16
		reason             string
	}{
		{
			name:               "真实设备DNY帧1_获取服务器时间",
			hexData:            "444e590900f36ca2040200120d03",
			expectedValid:      true,
			expectedPhysicalID: 0x04A26CF3,
			expectedCommand:    0x12,
			expectedMessageID:  0x0002,
			expectedChecksum:   0x030D,
			reason:             "真实设备发送的获取服务器时间命令",
		},
		{
			name:               "真实设备DNY帧2_设备注册",
			hexData:            "444e595000f36ca20403001168020220fc58681f07383938363034443931363233393034383832393755000038363434353230363937363234373256312e302e30302e3030303030302e3036313600000000002611",
			expectedValid:      true,
			expectedPhysicalID: 0x04A26CF3,
			expectedCommand:    0x11,
			expectedMessageID:  0x0003,
			expectedChecksum:   0x1126,
			reason:             "真实设备发送的注册命令，包含ICCID和版本信息",
		},
		{
			name:               "真实设备DNY帧3_状态上报",
			hexData:            "444e591d00cd28a2048008018002460902000000000000000000001e00315e00ac04",
			expectedValid:      true,
			expectedPhysicalID: 0x04A228CD,
			expectedCommand:    0x01,
			expectedMessageID:  0x0880,
			expectedChecksum:   0x04AC,
			reason:             "真实设备发送的状态上报命令",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Fatalf("十六进制解码失败: %v", err)
			}

			// 测试协议解析
			msg, err := ParseDNYProtocolData(data)
			if tc.expectedValid {
				if err != nil {
					t.Fatalf("协议解析应该成功: %s, 错误: %v", tc.reason, err)
				}
				if msg.MessageType != "standard" {
					t.Errorf("消息类型应为standard, 得到: %s", msg.MessageType)
				}
				if msg.PhysicalId != tc.expectedPhysicalID {
					t.Errorf("物理ID不匹配: 期望0x%08X, 得到0x%08X", tc.expectedPhysicalID, msg.PhysicalId)
				}
				if uint32(tc.expectedCommand) != msg.CommandId {
					t.Errorf("命令ID不匹配: 期望0x%02X, 得到0x%02X", tc.expectedCommand, msg.CommandId)
				}
				if tc.expectedMessageID != msg.MessageId {
					t.Errorf("消息ID不匹配: 期望0x%04X, 得到0x%04X", tc.expectedMessageID, msg.MessageId)
				}
				if tc.expectedChecksum != msg.Checksum {
					t.Errorf("校验和不匹配: 期望0x%04X, 得到0x%04X", tc.expectedChecksum, msg.Checksum)
				}
			} else {
				if err == nil {
					t.Errorf("协议解析应该失败: %s", tc.reason)
				}
			}

			// 测试DNY帧验证
			valid, err := ValidateDNYFrame(data)
			if tc.expectedValid {
				if !valid {
					t.Errorf("DNY帧验证应该通过")
				}
				if err != nil {
					t.Errorf("DNY帧验证不应有错误: %v", err)
				}
			} else {
				if valid {
					t.Errorf("DNY帧验证应该失败")
				}
				if err == nil {
					t.Errorf("DNY帧验证应该返回错误")
				}
			}
		})
	}
}

func TestLinkHeartbeatParsing_Standard(t *testing.T) {
	testCases := []struct {
		name     string
		hexData  string
		expected bool
		reason   string
	}{
		{
			name:     "标准Link心跳包",
			hexData:  "6c696e6b",
			expected: true,
			reason:   "标准4字节link心跳包",
		},
		{
			name:     "错误的心跳包内容",
			hexData:  "6c696e67", // "ling"
			expected: false,
			reason:   "内容不是link",
		},
		{
			name:     "长度错误的心跳包",
			hexData:  "6c696e", // "lin"
			expected: false,
			reason:   "长度不是4字节",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Fatalf("十六进制解码失败: %v", err)
			}

			msg, err := ParseDNYProtocolData(data)
			if tc.expected {
				if err != nil {
					t.Fatalf("Link心跳解析应该成功: %s, 错误: %v", tc.reason, err)
				}
				if msg.MessageType != "heartbeat_link" {
					t.Errorf("消息类型应为heartbeat_link, 得到: %s", msg.MessageType)
				}
			} else {
				// Link心跳解析失败时，应该尝试其他协议解析
				if err == nil && msg.MessageType == "heartbeat_link" {
					t.Errorf("消息类型不应为heartbeat_link")
				}
			}
		})
	}
}

func TestChecksumCalculation_Standard(t *testing.T) {
	testCases := []struct {
		name             string
		hexData          string
		expectedChecksum uint16
		reason           string
	}{
		{
			name:             "真实DNY帧校验和1",
			hexData:          "444e590900f36ca204020012", // 不包含校验和的部分
			expectedChecksum: 0x030D,
			reason:           "从包头DNY开始到校验和前的累加",
		},
		{
			name:             "真实DNY帧校验和2",
			hexData:          "444e595000f36ca20403001168020220fc58681f07383938363034443931363233393034383832393755000038363434353230363937363234373256312e302e30302e3030303030302e303631360000000000", // 不包含校验和的部分
			expectedChecksum: 0x1126,
			reason:           "长帧的校验和计算",
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
				t.Fatalf("校验和计算不应出错: %v", err)
			}
			if checksum != tc.expectedChecksum {
				t.Errorf("校验和计算错误: 期望0x%04X, 得到0x%04X - %s",
					tc.expectedChecksum, checksum, tc.reason)
			}
		})
	}
}

func TestProtocolUnification_Standard(t *testing.T) {
	// 测试所有ICCID验证函数的一致性
	testICCID := "898604D9162390488297"
	data := []byte(testICCID)

	// 所有ICCID验证函数应该返回相同结果
	result1 := isValidICCID(data)
	result2 := isValidICCIDStrict(data)
	result3 := IsValidICCIDPrefix(data)

	if !result1 {
		t.Error("isValidICCID应该返回true")
	}
	if !result2 {
		t.Error("isValidICCIDStrict应该返回true")
	}
	if !result3 {
		t.Error("IsValidICCIDPrefix应该返回true")
	}
	if result1 != result2 {
		t.Error("isValidICCID和isValidICCIDStrict结果应一致")
	}
	if result1 != result3 {
		t.Error("isValidICCID和IsValidICCIDPrefix结果应一致")
	}
}
