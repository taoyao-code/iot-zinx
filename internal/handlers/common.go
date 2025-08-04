package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"go.uber.org/zap"
)

// BaseHandler 基础处理器，提供公共方法
type BaseHandler struct {
	name string
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(name string) *BaseHandler {
	return &BaseHandler{name: name}
}

// ExtractDeviceData 从消息中提取设备数据
func (h *BaseHandler) ExtractDeviceData(msg *dny_protocol.Message, conn ziface.IConnection) (deviceID, physicalID, iccid string) {
	// 将物理ID转换为字符串
	physicalID = utils.FormatPhysicalID(msg.PhysicalId)

	// 从数据中提取ICCID（如果存在）
	if len(msg.Data) >= 20 {
		// 前20字节通常是ICCID
		iccid = strings.TrimSpace(string(msg.Data[:20]))
		// 清理非打印字符
		iccid = strings.Map(func(r rune) rune {
			if r >= 32 && r <= 126 {
				return r
			}
			return -1
		}, iccid)
	} else {
		iccid = ""
	}

	// 使用物理ID作为设备ID
	deviceID = physicalID

	return deviceID, physicalID, iccid
}

// BuildDeviceRegisterResponse 构建设备注册响应
func (h *BaseHandler) BuildDeviceRegisterResponse(physicalID string) []byte {
	// 根据DNY协议文档格式: DNY(3字节) + Length(2字节) + 物理ID(4字节) + 命令(1字节) + 消息ID(2字节) + 数据(N字节) + 校验和(2字节)

	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 准备数据内容
	dataContent := []byte{0x00} // 成功状态

	// 计算长度: 物理ID(4) + 命令(1) + 消息ID(2) + 数据(1) + 校验和(2) = 10字节
	contentLength := uint16(4 + 1 + 2 + len(dataContent) + 2)

	// 构建响应数据
	response := make([]byte, 0, 3+2+int(contentLength))

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 长度字段 (2字节，小端序)
	lengthBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lengthBytes, contentLength)
	response = append(response, lengthBytes...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 设备注册响应
	response = append(response, 0x20)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据
	response = append(response, dataContent...)

	// 校验和 (2字节，小端序) - 使用统一的校验函数
	checksum := dny_protocol.CalculateDNYChecksum(response)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	response = append(response, checksumBytes...)

	return response
}

// BuildHeartbeatResponse 构建心跳响应
func (h *BaseHandler) BuildHeartbeatResponse(physicalID string) []byte {
	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 准备数据内容
	dataContent := []byte{0x00} // 成功状态

	// 计算长度: 物理ID(4) + 命令(1) + 消息ID(2) + 数据(1) + 校验和(2) = 10字节
	contentLength := uint16(4 + 1 + 2 + len(dataContent) + 2)

	// 构建响应数据
	response := make([]byte, 0, 3+2+int(contentLength))

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 长度字段 (2字节，小端序)
	lengthBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lengthBytes, contentLength)
	response = append(response, lengthBytes...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 心跳响应
	response = append(response, 0x21)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据
	response = append(response, dataContent...)

	// 校验和 (2字节，小端序) - 使用统一的校验函数
	checksum := dny_protocol.CalculateDNYChecksum(response)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	response = append(response, checksumBytes...)

	return response
}

// BuildChargeControlResponse 构建充电控制响应
func (h *BaseHandler) BuildChargeControlResponse(physicalID string, success bool) []byte {
	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 准备数据内容
	status := byte(0x00)
	if !success {
		status = 0x01
	}
	dataContent := []byte{status}

	// 计算长度: 物理ID(4) + 命令(1) + 消息ID(2) + 数据(1) + 校验和(2) = 10字节
	contentLength := uint16(4 + 1 + 2 + len(dataContent) + 2)

	// 构建响应数据
	response := make([]byte, 0, 3+2+int(contentLength))

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 长度字段 (2字节，小端序)
	lengthBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lengthBytes, contentLength)
	response = append(response, lengthBytes...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 充电控制响应
	response = append(response, 0x82)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据
	response = append(response, dataContent...)

	// 校验和 (2字节，小端序) - 使用统一的校验函数
	checksum := dny_protocol.CalculateDNYChecksum(response)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	response = append(response, checksumBytes...)

	return response
}

// SendSuccessResponse 发送成功响应
func (h *BaseHandler) SendSuccessResponse(request ziface.IRequest, response []byte) {
	conn := request.GetConnection()
	if conn == nil {
		h.Log("连接不存在，无法发送响应")
		return
	}

	err := conn.SendMsg(1, response)
	if err != nil {
		h.Log("发送响应失败: %v", err)
	}
}

// Log 日志记录
func (h *BaseHandler) Log(format string, args ...interface{}) {
	logger.Info("Handler",
		zap.String("component", h.name),
		zap.String("message", fmt.Sprintf(format, args...)),
	)
}

// ParseAndValidateMessage 统一的协议解析和验证方法
// 消除各个handler中重复的ParseDNYMessage+ValidateMessage模式
func (h *BaseHandler) ParseAndValidateMessage(request ziface.IRequest) (*dny_protocol.ParsedMessage, error) {
	// 使用统一的协议解析
	parsedMsg := dny_protocol.ParseDNYMessage(request.GetData())

	// 🔧 修复：优化错误处理 - 对于扩展消息类型，不进行严格验证
	if parsedMsg.Error != nil {
		// 检查是否是未知消息类型，如果是则允许通过
		if strings.Contains(parsedMsg.Error.Error(), "unknown message type") {
			// 对于未知消息类型，清除错误，允许继续处理
			parsedMsg.Error = nil
			h.Log("收到未知消息类型，使用扩展处理: 0x%02X", parsedMsg.Command)
		} else {
			h.Log("消息解析失败: %v", parsedMsg.Error)
			return nil, fmt.Errorf("message parsing failed: %w", parsedMsg.Error)
		}
	}

	// 对于扩展消息类型，跳过严格验证
	if h.isExtendedMessageType(parsedMsg.MessageType) {
		h.Log("跳过扩展消息类型的严格验证: %s", dny_protocol.GetMessageTypeName(parsedMsg.MessageType))
		return parsedMsg, nil
	}

	// 对于标准消息类型，进行正常验证
	if err := dny_protocol.ValidateMessage(parsedMsg); err != nil {
		h.Log("消息验证失败: %v", err)
		return nil, fmt.Errorf("message validation failed: %w", err)
	}

	return parsedMsg, nil
}

// isExtendedMessageType 检查是否为扩展消息类型
func (h *BaseHandler) isExtendedMessageType(msgType dny_protocol.MessageType) bool {
	switch msgType {
	case dny_protocol.MsgTypeExtendedCommand,
		dny_protocol.MsgTypeExtHeartbeat1, dny_protocol.MsgTypeExtHeartbeat2, dny_protocol.MsgTypeExtHeartbeat3,
		dny_protocol.MsgTypeExtHeartbeat4, dny_protocol.MsgTypeExtHeartbeat5, dny_protocol.MsgTypeExtHeartbeat6,
		dny_protocol.MsgTypeExtHeartbeat7, dny_protocol.MsgTypeExtHeartbeat8,
		dny_protocol.MsgTypeExtCommand1, dny_protocol.MsgTypeExtCommand2, dny_protocol.MsgTypeExtCommand3, dny_protocol.MsgTypeExtCommand4,
		dny_protocol.MsgTypeExtStatus1, dny_protocol.MsgTypeExtStatus2, dny_protocol.MsgTypeExtStatus3,
		dny_protocol.MsgTypeExtStatus4, dny_protocol.MsgTypeExtStatus5, dny_protocol.MsgTypeExtStatus6,
		dny_protocol.MsgTypeExtStatus8, dny_protocol.MsgTypeExtStatus9,
		dny_protocol.MsgTypeExtStatus10, dny_protocol.MsgTypeExtStatus11, dny_protocol.MsgTypeExtStatus12,
		dny_protocol.MsgTypeExtStatus13, dny_protocol.MsgTypeExtStatus14, dny_protocol.MsgTypeExtStatus15,
		dny_protocol.MsgTypeExtStatus16, dny_protocol.MsgTypeExtStatus17, dny_protocol.MsgTypeExtStatus18,
		dny_protocol.MsgTypeExtStatus19, dny_protocol.MsgTypeExtStatus20:
		return true
	default:
		return false
	}
}

// ValidateMessageType 验证消息类型是否符合预期
func (h *BaseHandler) ValidateMessageType(parsedMsg *dny_protocol.ParsedMessage, expectedType dny_protocol.MessageType) error {
	if parsedMsg.MessageType != expectedType {
		err := fmt.Errorf("错误的消息类型: %s, 期望: %s",
			dny_protocol.GetMessageTypeName(parsedMsg.MessageType),
			dny_protocol.GetMessageTypeName(expectedType))
		h.Log("%s", err.Error())
		return err
	}
	return nil
}

// ValidateMessageTypes 验证消息类型是否符合预期的多个类型之一
func (h *BaseHandler) ValidateMessageTypes(parsedMsg *dny_protocol.ParsedMessage, expectedTypes ...dny_protocol.MessageType) error {
	for _, expectedType := range expectedTypes {
		if parsedMsg.MessageType == expectedType {
			return nil
		}
	}

	// 构建期望类型的字符串
	var expectedNames []string
	for _, expectedType := range expectedTypes {
		expectedNames = append(expectedNames, dny_protocol.GetMessageTypeName(expectedType))
	}

	err := fmt.Errorf("错误的消息类型: %s, 期望: %s",
		dny_protocol.GetMessageTypeName(parsedMsg.MessageType),
		strings.Join(expectedNames, " 或 "))
	h.Log("%s", err.Error())
	return err
}

// ExtractDeviceIDFromMessage 从解析的消息中提取设备ID
func (h *BaseHandler) ExtractDeviceIDFromMessage(parsedMsg *dny_protocol.ParsedMessage) string {
	return utils.FormatPhysicalID(parsedMsg.PhysicalID)
}

// UpdateDeviceStatus 更新设备状态
func (h *BaseHandler) UpdateDeviceStatus(deviceID string, status string, conn ziface.IConnection) {
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		h.Log("设备 %s 不存在，无法更新状态", deviceID)
		return
	}

	device.SetStatus(status)
	device.SetConnectionID(uint32(conn.GetConnID()))
	storage.GlobalDeviceStore.Set(deviceID, device)

	h.Log("设备 %s 状态更新为 %s", deviceID, status)
}

// CreateNewDevice 创建新设备
func (h *BaseHandler) CreateNewDevice(deviceID, physicalID, iccid string, conn ziface.IConnection) *storage.DeviceInfo {
	device := storage.NewDeviceInfo(deviceID, physicalID, iccid)
	device.SetStatus(storage.StatusOnline)
	device.SetConnectionID(uint32(conn.GetConnID()))

	storage.GlobalDeviceStore.Set(deviceID, device)

	h.Log("新设备注册: ID=%s, PhysicalID=%s, ICCID=%s", deviceID, physicalID, iccid)

	return device
}

// HexDump 十六进制转储
func (h *BaseHandler) HexDump(data []byte) string {
	return hex.EncodeToString(data)
}
