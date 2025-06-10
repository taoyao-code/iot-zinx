package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// parseFrame 解析DNY协议帧的核心函数
// 根据TLV简洁设计模式，将原始字节流转换为结构化的DecodedDNYFrame对象
func parseFrame(conn ziface.IConnection, data []byte) (*DecodedDNYFrame, error) {
	decodedFrame := &DecodedDNYFrame{
		RawData:    data,
		Connection: conn,
		FrameType:  FrameTypeUnknown,
	}

	// 1. 尝试识别特殊消息 (ICCID, "link")
	// 注意：特殊消息的识别应具有明确的、不易与标准帧混淆的特征。

	// 检查是否为"link"心跳消息
	if bytes.Equal(data, []byte("link")) {
		decodedFrame.FrameType = FrameTypeLinkHeartbeat
		return decodedFrame, nil
	}

	// 检查是否为ICCID消息 - 支持十六进制编码的ICCID
	if iccid, ok := extractICCID(data); ok {
		decodedFrame.FrameType = FrameTypeICCID
		decodedFrame.ICCIDValue = iccid

		// 强制性调试：输出到stderr
		fmt.Printf("🔍 DEBUG: ICCID识别成功! iccid=%s, dataHex=%x\n", iccid, data)
		logger.WithFields(logrus.Fields{
			"dataHex":   fmt.Sprintf("%x", data),
			"dataStr":   string(data),
			"iccid":     iccid,
			"frameType": "FrameTypeICCID",
		}).Info("解码器：识别到ICCID帧")
		return decodedFrame, nil
	}

	// 2. 按标准DNY帧结构解析
	const minFrameLen = 14 // DNY包头(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	if len(data) < minFrameLen {
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = fmt.Sprintf("数据长度不足 %d, 实际长度 %d", minFrameLen, len(data))
		return decodedFrame, errors.New(decodedFrame.ErrorMessage)
	}

	// 包头验证
	if !(data[0] == 'D' && data[1] == 'N' && data[2] == 'Y') {
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = "无效的DNY包头"
		return decodedFrame, errors.New(decodedFrame.ErrorMessage)
	}
	decodedFrame.Header = make([]byte, 3)
	copy(decodedFrame.Header, data[0:3])

	// 解析长度字段 (小端)
	decodedFrame.LengthField = binary.LittleEndian.Uint16(data[3:5])

	// 校验帧实际长度是否与长度字段匹配
	// 长度字段值 = 物理ID(4) + 消息ID(2) + 命令(1) + 数据(n) + 校验(2)
	// 完整帧长 = 包头(3) + 长度字段(2) + 长度字段值
	expectedFrameLength := 3 + 2 + int(decodedFrame.LengthField)
	if len(data) != expectedFrameLength {
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = fmt.Sprintf("帧长度与长度字段不匹配：预期 %d, 实际 %d, 长度字段值 %d",
			expectedFrameLength, len(data), decodedFrame.LengthField)
		return decodedFrame, errors.New(decodedFrame.ErrorMessage)
	}

	// 解析固定字段 (小端)
	decodedFrame.RawPhysicalID = make([]byte, 4)
	copy(decodedFrame.RawPhysicalID, data[5:9])
	decodedFrame.PhysicalID = parseAndFormatPhysicalID(decodedFrame.RawPhysicalID)

	decodedFrame.MessageID = binary.LittleEndian.Uint16(data[9:11])
	decodedFrame.Command = data[11]

	// 解析数据载荷 Payload
	// 数据区长度 = LengthField - (物理ID长 + 消息ID长 + 命令长 + 校验长)
	payloadLength := int(decodedFrame.LengthField) - (4 + 2 + 1 + 2)
	if payloadLength < 0 {
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = "根据长度字段计算出的载荷长度为负"
		return decodedFrame, errors.New(decodedFrame.ErrorMessage)
	}

	payloadEndOffset := 12 + payloadLength
	decodedFrame.Payload = make([]byte, payloadLength)
	if payloadLength > 0 {
		copy(decodedFrame.Payload, data[12:payloadEndOffset])
	}

	// 解析校验和
	decodedFrame.Checksum = make([]byte, 2)
	copy(decodedFrame.Checksum, data[payloadEndOffset:payloadEndOffset+2])

	// CRC校验
	calculatedCRC := calculateDNYCrc(data[:payloadEndOffset])
	decodedFrame.IsChecksumValid = bytes.Equal(calculatedCRC, decodedFrame.Checksum)

	if !decodedFrame.IsChecksumValid {
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = "CRC校验失败"
		// 即使校验失败，也返回解析出的数据，上层决定如何处理
	} else {
		decodedFrame.FrameType = FrameTypeStandard
	}

	return decodedFrame, nil
}

// parseAndFormatPhysicalID 将原始物理ID转换为可读格式
func parseAndFormatPhysicalID(rawID []byte) string {
	if len(rawID) != 4 {
		return ""
	}

	// 转换为大端模式：小端 40 aa ce 04 -> 大端 04 ce aa 40
	// 最高字节是设备识别码，后3字节是设备编号
	deviceCode := rawID[3]
	deviceNumber := uint32(rawID[0]) | uint32(rawID[1])<<8 | uint32(rawID[2])<<16

	// 格式化为 "设备识别码-设备编号" 格式，例如："04-13544000"
	return fmt.Sprintf("%02x-%08d", deviceCode, deviceNumber)
}

// calculateDNYCrc 计算DNY协议的CRC校验和
func calculateDNYCrc(data []byte) []byte {
	var sum uint16 = 0
	for _, b := range data {
		sum += uint16(b)
	}

	// 返回校验和的低2字节（小端模式）
	checksum := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksum, sum)
	return checksum
}

// extractICCID 从数据中提取ICCID
// 根据协议文档：通讯模块连接上服务器后会发送SIM卡号（ICCID），以字符串方式发送
func extractICCID(data []byte) (string, bool) {
	dataStr := string(data)

	// 排除DNY协议包：检查是否以"DNY"开头
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		return "", false
	}

	// 尝试作为十六进制字符串解码（如：3839383630344439313632333930343838323937）
	if len(dataStr)%2 == 0 && len(dataStr) >= 38 && len(dataStr) <= 50 {
		if decoded, err := hex.DecodeString(dataStr); err == nil {
			decodedStr := string(decoded)
			// 验证解码后的字符串是否为有效ICCID（19-25位，支持十六进制字符）
			if len(decodedStr) >= 19 && len(decodedStr) <= 25 && IsAllDigits([]byte(decodedStr)) {
				return decodedStr, true
			}
		}
	}

	// 直接检查是否为ICCID格式（19-25位，支持十六进制字符A-F）
	if len(dataStr) >= 19 && len(dataStr) <= 25 && IsAllDigits([]byte(dataStr)) {
		return dataStr, true
	}

	// 修复：只有当数据确实匹配ICCID格式时才返回true
	// 之前的逻辑缺陷：总是在最后返回true，导致所有数据都被识别为ICCID
	return "", false
}

// validatePhysicalID 验证物理ID格式
func validatePhysicalID(physicalID string) bool {
	// 物理ID格式应该是 "XX-XXXXXXXX" (设备识别码-设备编号)
	if len(physicalID) != 11 || physicalID[2] != '-' {
		return false
	}

	// 验证设备识别码部分（前2位十六进制）
	if _, err := strconv.ParseUint(physicalID[:2], 16, 8); err != nil {
		return false
	}

	// 验证设备编号部分（后8位十进制）
	if _, err := strconv.ParseUint(physicalID[3:], 10, 32); err != nil {
		return false
	}

	return true
}

// CreateErrorFrame 创建错误帧
func CreateErrorFrame(conn ziface.IConnection, data []byte, errMsg string) *DecodedDNYFrame {
	return &DecodedDNYFrame{
		FrameType:    FrameTypeParseError,
		RawData:      data,
		Connection:   conn,
		ErrorMessage: errMsg,
	}
}

// CreateICCIDFrame 创建ICCID帧
func CreateICCIDFrame(conn ziface.IConnection, data []byte, iccid string) *DecodedDNYFrame {
	return &DecodedDNYFrame{
		FrameType:  FrameTypeICCID,
		RawData:    data,
		Connection: conn,
		ICCIDValue: iccid,
	}
}

// CreateHeartbeatFrame 创建心跳帧
func CreateHeartbeatFrame(conn ziface.IConnection, data []byte) *DecodedDNYFrame {
	return &DecodedDNYFrame{
		FrameType:  FrameTypeLinkHeartbeat,
		RawData:    data,
		Connection: conn,
	}
}
