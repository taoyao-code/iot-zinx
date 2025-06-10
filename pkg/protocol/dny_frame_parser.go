package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/aceld/zinx/ziface"
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

	// 检查是否为ICCID消息 - ICCID通常是纯数字字符串，长度在15-20之间
	if len(data) >= 15 && len(data) <= 20 && isAllDigits(data) {
		iccid, ok := extractICCID(data)
		if ok {
			decodedFrame.FrameType = FrameTypeICCID
			decodedFrame.ICCIDValue = iccid
			return decodedFrame, nil
		}
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
func extractICCID(data []byte) (string, bool) {
	// ICCID应该是纯数字字符串，长度通常在15-20之间
	if len(data) < 15 || len(data) > 20 {
		return "", false
	}

	// 检查是否全为数字字符
	if !isAllDigits(data) {
		return "", false
	}

	return string(data), true
}

// isAllDigits 检查字节数组是否全为数字字符
func isAllDigits(data []byte) bool {
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
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
