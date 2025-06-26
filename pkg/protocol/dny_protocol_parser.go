package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex" // 确保导入 encoding/hex
	"errors"
	"fmt"
	"strconv"
	"strings"

	// 使用正确的模块路径
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger" // 新增：导入logger包
	"github.com/sirupsen/logrus"                                   // 新增：导入logrus包
	// "github.com/bujia/pkg/util/conversion" // 暂时注释，待确认路径或移除依赖
	// "github.com/bujia/pkg/util/log" // 暂时注释
	// "github.com/bujia/pkg/util/string_util" // 暂时注释
)

const (
	HeaderDNY          = "DNY"
	HeaderLink         = "link"
	MinPacketLength    = 12 // DNY + Length(2) + PhysicalID(4) + MessageID(2) + Command(1) + Checksum(2)
	LinkPacketLength   = 4  // link
	PhysicalIDLength   = 4
	MessageIDLength    = 2
	CommandLength      = 1
	ChecksumLength     = 2
	PacketHeaderLength = 3
	DataLengthPos      = 3
	DataLengthBytes    = 2
)

// ParseDNYProtocolData 解析DNY协议数据，支持标准DNY帧和链路心跳
// 返回统一的 *dny_protocol.Message 结构
func ParseDNYProtocolData(data []byte) (*dny_protocol.Message, error) {
	// DEBUG: Log input to ParseDNYProtocolData
	logger.WithFields(logrus.Fields{
		"inputDataLen": len(data),
		"inputDataHex": hex.EncodeToString(data), // 修改：记录完整的十六进制数据
	}).Debug("ParseDNYProtocolData: 收到待解析数据") // 修改：日志级别调整为 Debug

	dataLen := len(data)
	msg := &dny_protocol.Message{RawData: data} // 存储原始数据

	if dataLen == 0 {
		msg.MessageType = "error"
		msg.ErrorMessage = "empty data packet"
		return msg, errors.New(msg.ErrorMessage)
	}

	// 🔧 修复：统一ICCID识别逻辑 - 符合ITU-T E.118标准
	// ICCID固定长度为20字节，十六进制字符(0-9,A-F)，以"89"开头
	if dataLen == constants.IOT_SIM_CARD_LENGTH && isValidICCIDStrict(data) {
		msg.MessageType = "iccid"
		msg.ICCIDValue = string(data) // 直接使用原始数据作为ICCID，符合文档描述
		return msg, nil
	}

	// 尝试解析为链路心跳 (4字节, "link")
	if dataLen == LinkPacketLength && string(data) == HeaderLink {
		msg.MessageType = "heartbeat_link"
		// msg.Id = constants.MsgIDLinkHeartbeat // 示例：可以为特殊消息定义MsgID
		return msg, nil
	}

	// 尝试解析为标准DNY协议帧
	if dataLen < MinPacketLength {
		msg.MessageType = "error"
		msg.ErrorMessage = fmt.Sprintf("packet too short for DNY frame: %d bytes", dataLen)
		return msg, errors.New(msg.ErrorMessage)
	}

	msg.PacketHeader = string(data[:PacketHeaderLength])
	if msg.PacketHeader != HeaderDNY {
		msg.MessageType = "error"
		msg.ErrorMessage = fmt.Sprintf("invalid packet header: expected '%s', got '%s'", HeaderDNY, msg.PacketHeader)
		return msg, errors.New(msg.ErrorMessage)
	}

	declaredDataLen := binary.LittleEndian.Uint16(data[DataLengthPos : DataLengthPos+DataLengthBytes])
	// 修正：expectedTotalPacketLength 的计算。declaredDataLen (协议中的“长度”字段)
	// 已经包含了 PhysicalID, MessageID, Command, Data 和 Checksum 的总长度。
	// 因此，整个数据包的实际总长度是 包头(3) + 长度字段本身(2) + declaredDataLen。
	// 🔧 修复：根据真实设备数据分析，长度字段包含校验和
	// 长度字段的值 = 物理ID(4) + 消息ID(2) + 命令(1) + 数据(n) + 校验和(2)
	// 总包长度 = 包头"DNY"(3) + 长度字段(2) + 长度字段的值
	expectedTotalPacketLength := PacketHeaderLength + DataLengthBytes + int(declaredDataLen)

	if dataLen != expectedTotalPacketLength {
		msg.MessageType = "error"
		msg.ErrorMessage = fmt.Sprintf("packet length mismatch: declared content length %d (physicalID+msgID+cmd+data) implies total %d, but got %d. Input data may be truncated or malformed.", declaredDataLen, expectedTotalPacketLength, dataLen)
		return msg, errors.New(msg.ErrorMessage)
	}

	// contentStart 指向 PhysicalID 的开始
	contentStart := PacketHeaderLength + DataLengthBytes
	// contentAndChecksumEnd 指向整个 DNY 帧的末尾（即校验和之后）
	contentAndChecksumEnd := expectedTotalPacketLength
	// checksumStart 指向校验和字段的开始
	checksumStart := contentAndChecksumEnd - ChecksumLength

	// 提取校验和
	expectedChecksum := binary.LittleEndian.Uint16(data[checksumStart:contentAndChecksumEnd])

	// 🔧 修复：根据真实设备验证，校验和计算从包头"DNY"开始到校验和前的所有字节
	dataForChecksum := data[0:checksumStart]
	actualChecksum, err := CalculatePacketChecksumInternal(dataForChecksum)
	if err != nil {
		msg.MessageType = "error"
		msg.ErrorMessage = fmt.Sprintf("checksum calculation error: %v", err)
		return msg, err
	}

	msg.Checksum = actualChecksum
	if actualChecksum != expectedChecksum {
		msg.MessageType = "error"
		msg.ErrorMessage = fmt.Sprintf("checksum mismatch: expected %04X, got %04X", expectedChecksum, actualChecksum)
		// 即使校验和错误，也继续解析其他字段，但标记为错误类型
	}

	// contentBytes 是 PhysicalID, MessageID, Command, Data 的部分
	// 其结束位置是 checksumStart
	contentBytes := data[contentStart:checksumStart]

	if len(contentBytes) < PhysicalIDLength+MessageIDLength+CommandLength {
		newErrorMsg := fmt.Sprintf("content too short: %d bytes, needs at least %d for headers", len(contentBytes), PhysicalIDLength+MessageIDLength+CommandLength)
		if msg.MessageType == "error" { // 如果已有错误信息，附加新错误
			msg.ErrorMessage = fmt.Sprintf("%s; %s", msg.ErrorMessage, newErrorMsg)
		} else {
			msg.MessageType = "error"
			msg.ErrorMessage = newErrorMsg
		}
		return msg, errors.New(newErrorMsg) // 返回最新的主要错误
	}

	msg.PhysicalId = binary.LittleEndian.Uint32(contentBytes[:PhysicalIDLength])
	msg.MessageId = binary.LittleEndian.Uint16(contentBytes[PhysicalIDLength : PhysicalIDLength+MessageIDLength])
	msg.CommandId = uint32(contentBytes[PhysicalIDLength+MessageIDLength])
	msg.Id = msg.CommandId // Zinx MsgID 映射自 DNY Command ID

	payloadStart := PhysicalIDLength + MessageIDLength + CommandLength
	if len(contentBytes) > payloadStart {
		msg.Data = contentBytes[payloadStart:]
	} else {
		msg.Data = []byte{}
	}
	msg.DataLen = uint32(len(msg.Data))

	if msg.MessageType == "" { // 如果之前没有错误，则为标准消息
		msg.MessageType = "standard"
	}

	// 如果msg.MessageType是"error"但之前没有返回error, 表示校验和错误但解析继续
	if msg.MessageType == "error" && err == nil {
		return msg, errors.New(msg.ErrorMessage)
	}

	return msg, nil
}

// CalculatePacketChecksumInternal 是 CalculatePacketChecksum 的内部版本，避免循环依赖或公开不必要的接口
// 🔧 修复：dataFrame 参数应为从物理ID开始，直到数据内容结束的部分（不包括包头、长度字段和校验和）
// 根据实际测试：校验和只计算物理ID + 消息ID + 命令 + 数据部分
func CalculatePacketChecksumInternal(dataFrame []byte) (uint16, error) {
	// DEBUG: Log input to CalculatePacketChecksumInternal
	logger.WithFields(logrus.Fields{
		"dataFrameLen": len(dataFrame),
		"dataFrameHex": fmt.Sprintf("%.100x", dataFrame), // 最多显示前100字节
	}).Trace("CalculatePacketChecksumInternal: 收到待计算校验和的数据帧")

	if len(dataFrame) == 0 {
		return 0, errors.New("data frame for checksum calculation is empty")
	}

	var sum uint16
	for _, b := range dataFrame { // 从包头"DNY"开始计算到数据内容结束
		sum += uint16(b)
	}
	return sum, nil
}

// BuildDNYResponsePacketUnified 使用统一的 dny_protocol.Message 构建DNY响应数据包
func BuildDNYResponsePacketUnified(msg *dny_protocol.Message) ([]byte, error) {
	// 根据协议，“长度”字段的值应为 PhysicalID(4) + MessageID(2) + 命令(1) + 数据(n) + 校验(2) 的总和
	contentLen := uint16(PhysicalIDLength + MessageIDLength + CommandLength + len(msg.Data) + ChecksumLength)
	// 之前的错误： contentLen := PhysicalIDLength + MessageIDLength + CommandLength + len(msg.Data)

	if contentLen > 0xFFFF { // 理论上 uint16 最大值就是0xFFFF，但这里是检查计算后的 contentLen 是否超出了协议本身允许的最大包长限制（如果有的话，但协议文档是256字节，这里用0xFFFF作为uint16的自然上限）
		// 协议规定每包最多256字节，指的是“长度”字段声明的这部分内容。
		// 3(DNY) + 2(LenField) + 256 = 261.
		// 此处 contentLen 是“长度”字段的值，其最大为256.
		if contentLen > 256 {
			return nil, errors.New("payload too large for DNY packet (max content length 256 bytes)")
		}
	}

	packet := new(bytes.Buffer)
	packet.WriteString(HeaderDNY)
	binary.Write(packet, binary.LittleEndian, uint16(contentLen))

	checksumContent := new(bytes.Buffer)
	binary.Write(checksumContent, binary.LittleEndian, msg.PhysicalId)
	binary.Write(checksumContent, binary.LittleEndian, msg.MessageId)
	checksumContent.WriteByte(byte(msg.CommandId))
	checksumContent.Write(msg.Data)

	packet.Write(checksumContent.Bytes())

	// 🔧 修复：为了与解析时保持一致，校验和计算应该包含包头和长度字段
	// 构建完整的待校验数据：包头 + 长度字段 + 内容
	fullDataForChecksum := new(bytes.Buffer)
	fullDataForChecksum.WriteString(HeaderDNY)
	binary.Write(fullDataForChecksum, binary.LittleEndian, uint16(contentLen))
	fullDataForChecksum.Write(checksumContent.Bytes())
	dataForChecksum := fullDataForChecksum.Bytes()

	logger.WithFields(logrus.Fields{
		"dataForChecksum": hex.EncodeToString(dataForChecksum),
	}).Info("用于计算校验和的数据")

	checksum, err := CalculatePacketChecksumInternal(dataForChecksum)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum for unified packet: %w", err)
	}
	binary.Write(packet, binary.LittleEndian, checksum)

	return packet.Bytes(), nil
}

// ParseDevicePhysicalID 解析设备物理ID字符串 (复用之前的逻辑)
func ParseDevicePhysicalID(physicalIDStr string) (dny_protocol.PhysicalIdInfo, error) {
	var info dny_protocol.PhysicalIdInfo
	cleanIDStr := strings.TrimPrefix(physicalIDStr, "DNY-")
	if len(cleanIDStr) != 10 {
		return info, fmt.Errorf("invalid physical ID format: %s. Expected 10 digits after 'DNY-'", physicalIDStr)
	}
	typeCodeStr := cleanIDStr[:2]
	typeCode, err := strconv.ParseUint(typeCodeStr, 10, 8)
	if err != nil {
		return info, fmt.Errorf("invalid type code in physical ID '%s': %w", typeCodeStr, err)
	}
	info.TypeCode = byte(typeCode)
	numberStr := cleanIDStr[2:]
	number, err := strconv.ParseUint(numberStr, 10, 32)
	if err != nil {
		return info, fmt.Errorf("invalid number in physical ID '%s': %w", numberStr, err)
	}
	info.Number = uint32(number)
	return info, nil
}

// FormatDNYCommandData 格式化DNY命令和数据用于日志记录 - 使用统一的命令注册表
func FormatDNYCommandData(commandID byte, data []byte, direction string, physicalID uint32, messageID uint16) string {
	cmdName := constants.GetCommandName(uint8(commandID))
	dataHex := ""
	if len(data) > 0 {
		dataHex = hex.EncodeToString(data)
	}
	return fmt.Sprintf("[%s] PhysicalID: %d, MsgID: %d, Cmd: 0x%02X (%s), Data: %s",
		direction, physicalID, messageID, commandID, cmdName, dataHex)
}

// LogDNYMessage 记录DNY消息的详细信息
// 注意：由于 github.com/bujia/pkg 下的包路径问题，部分高级日志格式化功能已简化或移除。
// 待相关依赖路径确认后可恢复。
func LogDNYMessage(msg *dny_protocol.Message, direction string, connectionID uint64) {
	if msg == nil {
		// log.Debug(fmt.Sprintf("[%s] ConnID: %d, Received nil DNY message", direction, connectionID)) // 依赖 log
		fmt.Printf("[%s] ConnID: %d, Received nil DNY message\n", direction, connectionID) // 使用标准库打印
		return
	}

	var logMsg strings.Builder
	fmt.Fprintf(&logMsg, "[%s] ConnID: %d, Type: %s", direction, connectionID, msg.MessageType)

	switch msg.MessageType {
	case "standard":
		cmdName := constants.GetCommandName(uint8(msg.CommandId))
		fmt.Fprintf(&logMsg, ", PhysicalID: %d, DNYMsgID: %d, DNYCmd: 0x%02X (%s)", msg.PhysicalId, msg.MessageId, byte(msg.CommandId), cmdName)
		if msg.DataLen > 0 {
			fmt.Fprintf(&logMsg, ", DataLen: %d, Data: %s", msg.DataLen, hex.EncodeToString(msg.Data))
		}
		fmt.Fprintf(&logMsg, ", Checksum: %04X", msg.Checksum)
		if msg.RawData != nil {
			// fmt.Fprintf(&logMsg, ", Raw: %s", string_util.BytesToHexStringWithSpaces(msg.RawData)) // 依赖 string_util
			fmt.Fprintf(&logMsg, ", Raw: %s", hex.EncodeToString(msg.RawData)) // 使用标准库hex
		}
	case "iccid":
		fmt.Fprintf(&logMsg, ", ICCID: %s", msg.ICCIDValue)
		if msg.RawData != nil {
			// fmt.Fprintf(&logMsg, ", Raw: %s", conversion.BytesToReadableString(msg.RawData)) // 依赖 conversion
			fmt.Fprintf(&logMsg, ", Raw: %s", string(msg.RawData)) // 直接转为string尝试
		}
	case "heartbeat_link":
		fmt.Fprintf(&logMsg, ", Raw: %s", string(msg.RawData))
	case "error":
		fmt.Fprintf(&logMsg, ", Error: %s", msg.ErrorMessage)
		if msg.RawData != nil {
			// fmt.Fprintf(&logMsg, ", Raw: %s", string_util.BytesToHexStringWithSpaces(msg.RawData)) // 依赖 string_util
			fmt.Fprintf(&logMsg, ", Raw: %s", hex.EncodeToString(msg.RawData)) // 使用标准库hex
		}
	default:
		if msg.RawData != nil {
			// fmt.Fprintf(&logMsg, ", Raw: %s", string_util.BytesToHexStringWithSpaces(msg.RawData)) // 依赖 string_util
			fmt.Fprintf(&logMsg, ", Raw: %s", hex.EncodeToString(msg.RawData)) // 使用标准库hex
		}
	}

	// log.Debug(logMsg.String()) // 依赖 log
	fmt.Println(logMsg.String()) // 使用标准库打印
}

// IsSpecialMessage 检查是否为特殊消息类型（ICCID, link等）
func IsSpecialMessage(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	dataStr := string(data)

	// 检查是否为ICCID（数字字符串，通常20位）
	if isValidICCID(data) && len(data) == constants.IOT_SIM_CARD_LENGTH {
		return true
	}

	// 检查是否为link心跳
	if strings.TrimSpace(dataStr) == constants.IOT_LINK_HEARTBEAT {
		return true
	}

	return false
}

// 🔧 已删除过时的isAllDigits函数，统一使用isValidICCIDStrict进行ICCID验证

// isValidICCID 检查字节数组是否为有效的ICCID格式
// 🔧 修复：统一使用严格验证逻辑，符合ITU-T E.118标准
func isValidICCID(data []byte) bool {
	return isValidICCIDStrict(data)
}

// IsValidICCIDPrefix 检查数据是否符合ICCID前缀格式（为兼容文档中的函数名）
// 🔧 修复：统一使用严格验证逻辑，确保所有ICCID验证函数返回一致结果
func IsValidICCIDPrefix(data []byte) bool {
	return isValidICCIDStrict(data)
}

// 🔧 修复ICCID验证函数
// isValidICCIDStrict 严格验证ICCID格式 - 符合ITU-T E.118标准
// ICCID固定长度为20字节，十六进制字符(0-9,A-F)，以"89"开头
func isValidICCIDStrict(data []byte) bool {
	if len(data) != constants.IOT_SIM_CARD_LENGTH {
		return false
	}

	// 转换为字符串进行验证
	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头（ITU-T E.118标准，电信行业标识符）
	if dataStr[:2] != "89" {
		return false
	}

	// 必须全部为十六进制字符（0-9, A-F, a-f）
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}

// ValidateDNYFrame 验证DNY协议帧的完整性和校验和
// 根据文档要求，这是DNY协议解析的核心验证函数
func ValidateDNYFrame(frameData []byte) (bool, error) {
	if len(frameData) < MinPacketLength {
		return false, fmt.Errorf("frame too short: %d bytes, minimum required: %d", len(frameData), MinPacketLength)
	}

	// 检查包头
	if string(frameData[:3]) != HeaderDNY {
		return false, fmt.Errorf("invalid header: expected 'DNY', got '%s'", string(frameData[:3]))
	}

	// 解析长度字段
	declaredLength := binary.LittleEndian.Uint16(frameData[3:5])
	// 🔧 修复：长度字段包含校验和
	expectedTotalLength := 3 + 2 + int(declaredLength) // DNY(3) + Length(2) + Content(declaredLength，包含校验和)

	if len(frameData) != expectedTotalLength {
		return false, fmt.Errorf("length mismatch: declared %d, actual frame %d, expected total %d",
			declaredLength, len(frameData), expectedTotalLength)
	}

	// 计算并验证校验和
	contentEnd := len(frameData) - ChecksumLength
	expectedChecksum := binary.LittleEndian.Uint16(frameData[contentEnd:])

	actualChecksum, err := CalculatePacketChecksumInternal(frameData[0:contentEnd])
	if err != nil {
		return false, fmt.Errorf("checksum calculation failed: %v", err)
	}

	if actualChecksum != expectedChecksum {
		return false, fmt.Errorf("checksum mismatch: expected 0x%04X, got 0x%04X", expectedChecksum, actualChecksum)
	}

	return true, nil
}

// SplitPacketsFromBuffer 从字节缓冲区中分割出完整的数据包
// 支持处理ICCID、DNY协议包、link心跳包的混合数据流
// 返回：完整数据包列表、剩余未完成数据、错误信息
func SplitPacketsFromBuffer(buffer []byte) ([][]byte, []byte, error) {
	if len(buffer) == 0 {
		return nil, nil, nil
	}

	var packets [][]byte
	offset := 0
	bufferLen := len(buffer)

	logger.WithFields(logrus.Fields{
		"bufferLen": bufferLen,
		"bufferHex": fmt.Sprintf("%.200x", buffer), // 显示前200字节用于调试
	}).Debug("SplitPacketsFromBuffer: 开始分割数据包")

	for offset < bufferLen {
		// 检查剩余数据长度
		remaining := bufferLen - offset
		if remaining == 0 {
			break
		}

		// 尝试识别ICCID (20字节，以"89"开头)
		if remaining >= constants.IOT_SIM_CARD_LENGTH {
			candidate := buffer[offset : offset+constants.IOT_SIM_CARD_LENGTH]
			if isValidICCIDStrict(candidate) {
				packets = append(packets, candidate)
				offset += constants.IOT_SIM_CARD_LENGTH
				logger.WithFields(logrus.Fields{
					"packetType": "iccid",
					"packetLen":  constants.IOT_SIM_CARD_LENGTH,
					"iccid":      string(candidate),
				}).Debug("SplitPacketsFromBuffer: 提取ICCID包")
				continue
			}
		}

		// 尝试识别link心跳包 (4字节 "link")
		if remaining >= LinkPacketLength {
			candidate := buffer[offset : offset+LinkPacketLength]
			if string(candidate) == HeaderLink {
				packets = append(packets, candidate)
				offset += LinkPacketLength
				logger.WithFields(logrus.Fields{
					"packetType": "link",
					"packetLen":  LinkPacketLength,
				}).Debug("SplitPacketsFromBuffer: 提取link心跳包")
				continue
			}
		}

		// 尝试识别DNY协议包
		if remaining >= PacketHeaderLength {
			// 检查DNY包头
			if string(buffer[offset:offset+PacketHeaderLength]) == HeaderDNY {
				// 检查是否有足够数据读取长度字段
				if remaining < PacketHeaderLength+DataLengthBytes {
					// 数据不完整，返回剩余数据
					logger.WithFields(logrus.Fields{
						"remaining":   remaining,
						"needMinimum": PacketHeaderLength + DataLengthBytes,
						"packetType":  "dny_incomplete_header",
					}).Debug("SplitPacketsFromBuffer: DNY包头不完整，保留剩余数据")
					break
				}

				// 读取长度字段
				lengthStart := offset + PacketHeaderLength
				declaredLength := binary.LittleEndian.Uint16(buffer[lengthStart : lengthStart+DataLengthBytes])
				totalPacketLength := PacketHeaderLength + DataLengthBytes + int(declaredLength)

				// 检查是否有完整的数据包
				if remaining < totalPacketLength {
					// 数据包不完整，返回剩余数据
					logger.WithFields(logrus.Fields{
						"remaining":         remaining,
						"totalPacketLength": totalPacketLength,
						"declaredLength":    declaredLength,
						"packetType":        "dny_incomplete_body",
					}).Debug("SplitPacketsFromBuffer: DNY包数据不完整，保留剩余数据")
					break
				}

				// 提取完整的DNY数据包
				packet := buffer[offset : offset+totalPacketLength]
				packets = append(packets, packet)
				offset += totalPacketLength

				logger.WithFields(logrus.Fields{
					"packetType":     "dny",
					"packetLen":      totalPacketLength,
					"declaredLength": declaredLength,
					"physicalIdHex":  fmt.Sprintf("%x", packet[5:9]), // PhysicalID位置
				}).Debug("SplitPacketsFromBuffer: 提取DNY协议包")
				continue
			}
		}

		// 无法识别的数据，跳过一个字节继续扫描
		logger.WithFields(logrus.Fields{
			"offset":       offset,
			"unrecognized": fmt.Sprintf("%02x", buffer[offset]),
			"contextHex":   fmt.Sprintf("%.20x", buffer[offset:min(offset+10, bufferLen)]),
		}).Warn("SplitPacketsFromBuffer: 跳过无法识别的字节")
		offset++
	}

	// 返回剩余未处理的数据
	var remainingData []byte
	if offset < bufferLen {
		remainingData = buffer[offset:]
		logger.WithFields(logrus.Fields{
			"remainingLen": len(remainingData),
			"remainingHex": fmt.Sprintf("%.100x", remainingData),
		}).Debug("SplitPacketsFromBuffer: 返回剩余未完成数据")
	}

	logger.WithFields(logrus.Fields{
		"totalPackets":   len(packets),
		"processedBytes": offset,
		"remainingBytes": len(remainingData),
	}).Debug("SplitPacketsFromBuffer: 分割完成")

	return packets, remainingData, nil
}

// ParseMultiplePackets 解析从缓冲区分割出的多个数据包
// 这是对外的主要接口，内部调用SplitPacketsFromBuffer和ParseDNYProtocolData
func ParseMultiplePackets(buffer []byte) ([]*dny_protocol.Message, []byte, error) {
	packets, remainingData, err := SplitPacketsFromBuffer(buffer)
	if err != nil {
		return nil, remainingData, fmt.Errorf("packet splitting failed: %w", err)
	}

	var messages []*dny_protocol.Message
	for i, packet := range packets {
		msg, parseErr := ParseDNYProtocolData(packet)
		if parseErr != nil {
			logger.WithFields(logrus.Fields{
				"packetIndex": i,
				"packetLen":   len(packet),
				"packetHex":   fmt.Sprintf("%.100x", packet),
				"error":       parseErr.Error(),
			}).Warn("ParseMultiplePackets: 单个数据包解析失败")
			// 继续处理其他包，不因单个包失败而中断整体处理
			continue
		}
		messages = append(messages, msg)
	}

	logger.WithFields(logrus.Fields{
		"inputBufferLen":     len(buffer),
		"splitPacketCount":   len(packets),
		"parsedMessageCount": len(messages),
		"remainingDataLen":   len(remainingData),
	}).Debug("ParseMultiplePackets: 多包解析完成")

	return messages, remainingData, nil
}
