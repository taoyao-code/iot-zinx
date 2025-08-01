package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
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
	physicalID = fmt.Sprintf("%08X", msg.PhysicalId)

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
	// 简单的DNY协议响应
	// 格式: 包头(3字节) + 物理ID(4字节) + 命令(1字节) + 消息ID(2字节) + 数据长度(2字节) + 数据(N字节) + 校验和(2字节)

	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 构建响应数据
	response := make([]byte, 0, 12)

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 设备注册响应
	response = append(response, 0x20)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据长度 (2字节，小端序) - 1字节数据
	response = append(response, []byte{0x01, 0x00}...)

	// 数据 (1字节) - 成功状态
	response = append(response, 0x00)

	// 校验和 (2字节，小端序) - 简单计算
	checksum := uint16(0)
	for _, b := range response[3:] { // 从物理ID开始计算
		checksum += uint16(b)
	}
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	response = append(response, checksumBytes...)

	return response
}

// BuildHeartbeatResponse 构建心跳响应
func (h *BaseHandler) BuildHeartbeatResponse(physicalID string) []byte {
	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 构建响应数据
	response := make([]byte, 0, 12)

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 心跳响应
	response = append(response, 0x21)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据长度 (2字节，小端序) - 1字节数据
	response = append(response, []byte{0x01, 0x00}...)

	// 数据 (1字节) - 成功状态
	response = append(response, 0x00)

	// 校验和 (2字节，小端序)
	checksum := uint16(0)
	for _, b := range response[3:] {
		checksum += uint16(b)
	}
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	response = append(response, checksumBytes...)

	return response
}

// BuildChargeControlResponse 构建充电控制响应
func (h *BaseHandler) BuildChargeControlResponse(physicalID string, success bool) []byte {
	physicalIDUint := uint32(0)
	fmt.Sscanf(physicalID, "%08X", &physicalIDUint)

	// 构建响应数据
	response := make([]byte, 0, 12)

	// 包头 "DNY"
	response = append(response, []byte("DNY")...)

	// 物理ID (4字节，小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalIDUint)
	response = append(response, idBytes...)

	// 命令 (1字节) - 充电控制响应
	response = append(response, 0x82)

	// 消息ID (2字节，小端序)
	response = append(response, []byte{0x00, 0x00}...)

	// 数据长度 (2字节，小端序) - 1字节数据
	response = append(response, []byte{0x01, 0x00}...)

	// 数据 (1字节) - 成功状态
	status := byte(0x00)
	if !success {
		status = 0x01
	}
	response = append(response, status)

	// 校验和 (2字节，小端序)
	checksum := uint16(0)
	for _, b := range response[3:] {
		checksum += uint16(b)
	}
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
	log.Printf("[%s] %s", h.name, fmt.Sprintf(format, args...))
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
