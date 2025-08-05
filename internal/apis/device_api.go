package apis

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"go.uber.org/zap"
)

// DeviceAPI 设备API
type DeviceAPI struct {
	connectionMonitor *handlers.ConnectionMonitor
}

// NewDeviceAPI 创建设备API
func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{}
}

// SetConnectionMonitor 设置连接监控器
func (api *DeviceAPI) SetConnectionMonitor(monitor *handlers.ConnectionMonitor) {
	api.connectionMonitor = monitor
}

// sendProtocolPacket 发送协议包到设备
func (api *DeviceAPI) sendProtocolPacket(deviceID string, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	if api.connectionMonitor == nil {
		return fmt.Errorf("连接监控器未初始化")
	}

	// 将物理ID转换为系统内部使用的十六进制格式
	hexDeviceID := fmt.Sprintf("%08X", physicalID)

	// 详细日志：记录发送前的状态
	logger.Info("准备发送协议包",
		zap.String("component", "device_api"),
		zap.String("device_id", deviceID),
		zap.String("hex_device_id", hexDeviceID),
		zap.Uint32("physical_id", physicalID),
		zap.Uint8("command", command),
		zap.Int("data_length", len(data)),
	)

	// 获取设备连接对象
	conn, exists := api.connectionMonitor.GetConnectionByDeviceId(hexDeviceID)
	if !exists {
		logger.Error("设备连接不存在",
			zap.String("component", "device_api"),
			zap.String("device_id", deviceID),
			zap.String("hex_device_id", hexDeviceID),
		)
		return fmt.Errorf("设备 %s 未连接", deviceID)
	}

	// 验证连接状态
	if conn == nil {
		logger.Error("连接对象为空",
			zap.String("component", "device_api"),
			zap.String("device_id", deviceID),
		)
		return fmt.Errorf("设备 %s 连接对象无效", deviceID)
	}

	// 获取连接详细信息进行验证
	connID := uint32(conn.GetConnID())
	remoteAddr := conn.RemoteAddr().String()

	logger.Info("找到设备连接",
		zap.String("component", "device_api"),
		zap.String("device_id", deviceID),
		zap.Uint32("conn_id", connID),
		zap.String("remote_addr", remoteAddr),
	)

	// 构建DNY协议包
	packet := dny_protocol.BuildDNYPacket(physicalID, messageID, command, data)

	// 通过TCP连接直接发送（绕过Zinx封装）
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		logger.Error("无法获取TCP连接",
			zap.String("component", "device_api"),
			zap.String("device_id", deviceID),
			zap.Uint32("conn_id", connID),
		)
		return fmt.Errorf("无法获取TCP连接")
	}

	_, err := tcpConn.Write(packet)
	if err != nil {
		logger.Error("发送协议包失败",
			zap.String("component", "device_api"),
			zap.String("device_id", deviceID),
			zap.Uint32("conn_id", connID),
			zap.Error(err),
		)
		return fmt.Errorf("发送协议包失败: %v", err)
	}

	// 记录发送成功
	logger.Info("协议包发送成功",
		zap.String("component", "device_api"),
		zap.String("device_id", deviceID),
		zap.Uint32("conn_id", connID),
		zap.Uint8("command", command),
		zap.Int("data_length", len(data)),
	)

	return nil
}

// generateMessageID 生成消息ID
func (api *DeviceAPI) generateMessageID() uint16 {
	return uint16(time.Now().Unix() & 0xFFFF)
}

// parseDeviceID 解析设备ID为物理ID
// 支持十进制和十六进制格式输入
func (api *DeviceAPI) parseDeviceID(deviceID string) (uint32, error) {
	// 首先尝试解析为十进制（实际环境中的常见格式）
	if decimalID, err := strconv.ParseUint(deviceID, 10, 32); err == nil {
		// 对于十进制输入，需要添加"04"前缀来匹配系统中的设备ID格式
		// 例如：10644723 -> 0x00A26CF3 -> 0x04A26CF3
		if decimalID <= 0xFFFFFF { // 确保不超过24位
			return uint32(0x04000000 | decimalID), nil
		}
		return uint32(decimalID), nil
	}

	// 如果十进制解析失败，尝试解析为十六进制（兼容现有格式）
	if hexID, err := strconv.ParseUint(deviceID, 16, 32); err == nil {
		return uint32(hexID), nil
	}

	return 0, fmt.Errorf("无效的设备ID格式: %s（支持十进制或十六进制格式）", deviceID)
}

// getDeviceByID 根据设备ID获取设备信息（支持十进制和十六进制输入）
func (api *DeviceAPI) getDeviceByID(deviceID string) (*storage.DeviceInfo, bool, error) {
	// 解析设备ID为物理ID
	physicalID, err := api.parseDeviceID(deviceID)
	if err != nil {
		return nil, false, err
	}

	// 将物理ID转换为系统内部使用的十六进制格式
	hexDeviceID := fmt.Sprintf("%08X", physicalID)

	// 从设备存储中获取设备信息
	device, exists := storage.GlobalDeviceStore.Get(hexDeviceID)
	return device, exists, nil
}
