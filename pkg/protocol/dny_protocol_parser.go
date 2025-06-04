package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// DNY协议解析常量和工具函数
// 本文件集中所有DNY协议解析相关功能，避免重复实现和不一致问题
// ============================================================================

// DNY协议标识常量
const (
	DNY_PROTOCOL_PREFIX  = "DNY"    // DNY协议前缀（二进制）
	DNY_HEX_PREFIX_LOWER = "444e59" // DNY协议前缀（小写十六进制）
	DNY_HEX_PREFIX_UPPER = "444E59" // DNY协议前缀（大写十六进制）
	DNY_MIN_BINARY_LEN   = 3        // DNY协议最小二进制长度
	DNY_MIN_HEX_LEN      = 6        // DNY协议最小十六进制长度
	DNY_MIN_PACKET_LEN   = 14       // DNY协议最小包长度
)

// 特殊消息ID常量
const (
	MSG_ID_UNKNOWN   = 0xFFFF // 未知消息ID
	MSG_ID_ICCID     = 0xFF01 // ICCID消息ID
	MSG_ID_HEARTBEAT = 0xFF02 // 心跳消息ID
)

// ICCID相关常量
const (
	ICCID_MIN_LEN = 19 // ICCID最小长度
	ICCID_MAX_LEN = 25 // ICCID最大长度
)

// 心跳消息常量
const (
	HEARTBEAT_MSG_LEN  = 4      // 心跳消息长度
	IOT_LINK_HEARTBEAT = "link" // link心跳字符串
)

// 连接属性键常量
const (
	PROP_DNY_PHYSICAL_ID    = "DNY_PhysicalID"    // 物理ID属性键
	PROP_DNY_MESSAGE_ID     = "DNY_MessageID"     // 消息ID属性键
	PROP_DNY_COMMAND        = "DNY_Command"       // 命令属性键
	PROP_DNY_CHECKSUM_VALID = "DNY_ChecksumValid" // 校验和有效性属性键
)

// 校验和计算方法常量
const (
	// 校验和计算方法
	CHECKSUM_METHOD_1 = 1 // 从物理ID开始计算(原始方法)
	CHECKSUM_METHOD_2 = 2 // 计算整个数据包

	// 默认使用的校验和计算方法
	DEFAULT_CHECKSUM_METHOD = CHECKSUM_METHOD_1
)

// 当前使用的校验和计算方法
var currentChecksumMethod = DEFAULT_CHECKSUM_METHOD

// SetChecksumMethod 设置校验和计算方法
func SetChecksumMethod(method int) {
	if method == CHECKSUM_METHOD_1 || method == CHECKSUM_METHOD_2 {
		currentChecksumMethod = method
	}
}

// GetChecksumMethod 获取当前校验和计算方法
func GetChecksumMethod() int {
	return currentChecksumMethod
}

// ============================================================================
// 核心解析函数 - 统一DNY协议解析接口
// ============================================================================

// ParseDNYProtocolData 统一的DNY协议解析函数
// 所有DNY协议解析必须使用这个函数，确保解析逻辑一致
func ParseDNYProtocolData(data []byte) (*dny_protocol.Message, error) {
	// 数据长度检查
	if len(data) < DNY_MIN_PACKET_LEN {
		return nil, fmt.Errorf("数据长度不足，至少需要%d字节，实际长度: %d", DNY_MIN_PACKET_LEN, len(data))
	}

	// 包头检查
	if string(data[0:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头，期望为DNY，实际为: %s", string(data[0:3]))
	}

	// 解析长度 (小端序)
	length := binary.LittleEndian.Uint16(data[3:5])

	// 检查数据长度是否完整
	totalLen := 5 + int(length)
	if len(data) < totalLen {
		return nil, fmt.Errorf("数据长度不足，期望长度: %d, 实际长度: %d", totalLen, len(data))
	}

	// 解析物理ID (小端序)
	physicalID := binary.LittleEndian.Uint32(data[5:9])

	// 解析消息ID (小端序)
	messageID := binary.LittleEndian.Uint16(data[9:11])

	// 解析命令
	command := data[11]

	// 解析数据部分
	dataLength := int(length) - 9 // 减去物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	var payload []byte
	if dataLength > 0 && len(data) >= 12+dataLength {
		payload = data[12 : 12+dataLength]
	} else {
		payload = []byte{}
	}

	// 解析校验和 (小端序)
	checksumPos := 12 + dataLength
	var checksum uint16
	if checksumPos+1 < len(data) {
		checksum = binary.LittleEndian.Uint16(data[checksumPos : checksumPos+2])
	}

	// 保存当前校验和计算方法
	originalMethod := currentChecksumMethod

	// 尝试方法1: 原始方法(从物理ID开始计算)
	SetChecksumMethod(CHECKSUM_METHOD_1)
	checksum1 := CalculatePacketChecksum(data[:checksumPos])
	isValid1 := (checksum1 == checksum)

	// 尝试方法2: 整个数据包
	SetChecksumMethod(CHECKSUM_METHOD_2)
	checksum2 := CalculatePacketChecksum(data[:checksumPos])
	isValid2 := (checksum2 == checksum)

	// 恢复原始方法
	SetChecksumMethod(originalMethod)

	// 判断校验和是否有效
	checksumValid := isValid1 || isValid2

	if !checksumValid {
		// 校验和验证失败，记录增强的日志但继续处理
		logger.WithFields(logrus.Fields{
			"command":          fmt.Sprintf("0x%02X", command),
			"expectedChecksum": fmt.Sprintf("0x%04X", checksum),
			"method1Checksum":  fmt.Sprintf("0x%04X", checksum1),
			"method1Valid":     isValid1,
			"method2Checksum":  fmt.Sprintf("0x%04X", checksum2),
			"method2Valid":     isValid2,
			"physicalID":       fmt.Sprintf("0x%08X", physicalID),
			"messageID":        messageID,
			"rawData":          hex.EncodeToString(data),
		}).Warn("DNY校验和验证失败，但仍继续处理")

		// 如果某种方法有效，建议更改默认方法
		if isValid1 && currentChecksumMethod != CHECKSUM_METHOD_1 {
			logger.Info("建议将校验和计算方法更改为方法1(从物理ID开始)")
		} else if isValid2 && currentChecksumMethod != CHECKSUM_METHOD_2 {
			logger.Info("建议将校验和计算方法更改为方法2(整个数据包)")
		}
	}

	// 创建dny_protocol.Message对象
	// 设置MsgID为命令码，这是处理器路由需要的
	dnyMsg := dny_protocol.NewMessage(uint32(command), physicalID, payload, messageID)

	// 保存原始数据
	if len(data) >= totalLen {
		dnyMsg.SetRawData(data[:totalLen])
	} else {
		dnyMsg.SetRawData(data)
	}

	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", command),
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  messageID,
		"dataLength": dataLength,
		"checksumOK": checksumValid,
	}).Debug("DNY协议解析成功")

	return dnyMsg, nil
}

// ============================================================================
// 特殊消息处理函数 - 集中处理特殊消息逻辑
// ============================================================================

// IsSpecialMessage 检查是否为特殊消息（ICCID或心跳）
// 所有特殊消息检测都应使用此函数
func IsSpecialMessage(data []byte) bool {
	dataLen := len(data)

	// 检查是否为ICCID
	if IsICCID(data, dataLen) {
		return true
	}

	// 检查是否为心跳消息
	if IsHeartbeat(data, dataLen) {
		return true
	}

	return false
}

// ParseSpecialMessage 解析特殊消息为dny_protocol.Message
// 当确认是特殊消息后，使用此函数生成统一格式的消息对象
func ParseSpecialMessage(data []byte) *dny_protocol.Message {
	dataLen := len(data)
	var msgID uint32 = MSG_ID_UNKNOWN

	// 检查是否为ICCID
	if IsICCID(data, dataLen) {
		msgID = MSG_ID_ICCID
		logger.WithFields(logrus.Fields{
			"msgType": "ICCID",
			"data":    string(data),
		}).Info("检测到ICCID特殊消息")
	} else if IsHeartbeat(data, dataLen) { // 检查是否为心跳消息
		msgID = MSG_ID_HEARTBEAT
		logger.WithFields(logrus.Fields{
			"msgType": "心跳",
			"data":    string(data),
		}).Info("检测到心跳特殊消息")
	}

	// 创建消息对象
	msg := dny_protocol.NewMessage(msgID, 0, data, 0)
	msg.SetRawData(data)

	return msg
}

// IsICCID 检查数据是否为ICCID
func IsICCID(data []byte, dataLen int) bool {
	return dataLen >= ICCID_MIN_LEN && dataLen <= ICCID_MAX_LEN && IsAllDigits(data)
}

// IsHeartbeat 检查数据是否为心跳消息
func IsHeartbeat(data []byte, dataLen int) bool {
	return dataLen == HEARTBEAT_MSG_LEN && string(data) == IOT_LINK_HEARTBEAT
}

// IsAllDigits 检查是否为合法的ICCID格式（数字和十六进制字符A-F）
func IsAllDigits(data []byte) bool {
	return strings.IndexFunc(string(data), func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') || (r >= 'a' && r <= 'f'))
	}) == -1
}

// ============================================================================
// 辅助函数 - 提供通用工具功能
// ============================================================================

// CalculatePacketChecksum 计算校验和
func CalculatePacketChecksum(data []byte) uint16 {
	// 检查数据是否足够长
	if len(data) < 5 {
		return 0
	}

	var checksum uint16

	switch currentChecksumMethod {
	case CHECKSUM_METHOD_1:
		// 方法1: 从物理ID位置(5)开始计算校验和，排除DNY前缀和长度字段
		for i := 5; i < len(data); i++ {
			checksum += uint16(data[i])
		}
	case CHECKSUM_METHOD_2:
		// 方法2: 计算整个数据包的校验和
		for i := 0; i < len(data); i++ {
			checksum += uint16(data[i])
		}
	default:
		// 默认使用方法1
		for i := 5; i < len(data); i++ {
			checksum += uint16(data[i])
		}
	}

	return checksum
}

// IsDNYProtocolData 检查数据是否符合DNY协议格式
func IsDNYProtocolData(data []byte) bool {
	// 检查最小长度
	if len(data) < DNY_MIN_PACKET_LEN {
		return false
	}

	// 检查包头是否为"DNY"
	if !bytes.HasPrefix(data, []byte(DNY_PROTOCOL_PREFIX)) {
		return false
	}

	// 解析数据长度字段
	dataLen := binary.LittleEndian.Uint16(data[3:5])
	totalLen := 5 + int(dataLen)

	// 检查实际长度是否匹配
	if len(data) < totalLen {
		return false
	}

	return true
}

// IsHexString 检查字节数组是否为有效的十六进制字符串
func IsHexString(data []byte) bool {
	// 检查是否为合适的十六进制长度
	if len(data) == 0 || len(data)%2 != 0 {
		return false
	}

	// 检查是否都是十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}

	return true
}
