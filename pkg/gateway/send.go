package gateway

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// SendCommandToDevice 发送命令到指定设备（统一发送路径）
func (g *DeviceGateway) SendCommandToDevice(deviceID string, command byte, data []byte) error {
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	// AP3000 发送节流：同设备命令间隔≥0.5秒
	g.throttleMu.Lock()
	if last, ok := g.lastSendByDevice[deviceID]; ok {
		if wait := 500*time.Millisecond - time.Since(last); wait > 0 {
			g.throttleMu.Unlock()
			time.Sleep(wait)
			g.throttleMu.Lock()
		}
	}
	g.lastSendByDevice[deviceID] = time.Now()
	g.throttleMu.Unlock()

	// 标准化设备ID
	processor := &utils.DeviceIDProcessor{}
	stdDeviceID, err := processor.SmartConvertDeviceID(deviceID)
	if err != nil {
		return fmt.Errorf("设备ID解析失败: %v", err)
	}

	conn, exists := g.tcpManager.GetConnectionByDeviceID(stdDeviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不在线", stdDeviceID)
	}

	// 验证设备会话存在
	_, sessionExists := g.tcpManager.GetSessionByDeviceID(stdDeviceID)
	if !sessionExists {
		return fmt.Errorf("设备会话不存在")
	}

	// 设备ID→PhysicalID
	expectedPhysicalID, err := utils.ParseDeviceIDToPhysicalID(stdDeviceID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 从设备信息中获取并校验PhysicalID
	device, deviceExists := g.tcpManager.GetDeviceByID(stdDeviceID)
	if !deviceExists {
		return fmt.Errorf("设备 %s 不存在", stdDeviceID)
	}

	sessionPhysicalID := device.PhysicalID
	if expectedPhysicalID != sessionPhysicalID {
		logger.WithFields(logrus.Fields{
			"deviceID":           stdDeviceID,
			"expectedPhysicalID": utils.FormatPhysicalID(expectedPhysicalID),
			"devicePhysicalID":   utils.FormatPhysicalID(sessionPhysicalID),
			"action":             "FIXING_PHYSICAL_ID_MISMATCH",
		}).Warn("🔧 检测到PhysicalID不匹配，正在修复Device数据")

		device.Lock()
		device.PhysicalID = expectedPhysicalID
		device.Unlock()
		if err := g.fixDeviceGroupPhysicalID(stdDeviceID, expectedPhysicalID); err != nil {
			logger.WithFields(logrus.Fields{"deviceID": stdDeviceID, "error": err}).Error("修复设备组PhysicalID失败")
		}
		logger.WithFields(logrus.Fields{"deviceID": stdDeviceID, "correctedPhysicalID": utils.FormatPhysicalID(expectedPhysicalID)}).Info("✅ PhysicalID不匹配已修复")
	}
	physicalID := expectedPhysicalID

	// 生成消息ID并构包
	messageID := pkg.Protocol.GetNextMessageID()
	builder := protocol.NewUnifiedDNYBuilder()
	dnyPacket := builder.BuildDNYPacket(physicalID, messageID, command, data)

	// 发送前校验
	if err := protocol.ValidateUnifiedDNYPacket(dnyPacket); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":   stdDeviceID,
			"physicalID": utils.FormatPhysicalID(physicalID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"command":    fmt.Sprintf("0x%02X", command),
			"reason":     err.Error(),
		}).Error("❌ DNY数据包校验失败，拒绝发送")
		return fmt.Errorf("DNY包校验失败: %w", err)
	}

	// 注册命令到 CommandManager（用于超时与重试管理）
	cmdMgr := network.GetCommandManager()
	if cmdMgr != nil {
		cmdMgr.RegisterCommand(conn, physicalID, messageID, uint8(command), data)
	}

	// 通过 UnifiedSender 发送（保持唯一发送路径）
	if err := pkg.Protocol.SendDNYPacket(conn, dnyPacket); err != nil {
		return fmt.Errorf("发送命令失败: %v", err)
	}

	// 记录命令元数据
	g.tcpManager.RecordDeviceCommand(stdDeviceID, command, len(data))

	// 成功日志（结构化）：符合 AP3000 日志规范
	logger.WithFields(logrus.Fields{
		"deviceID":   stdDeviceID,
		"physicalID": utils.FormatPhysicalID(physicalID),
		"msgID":      fmt.Sprintf("0x%04X", messageID),
		"cmd":        fmt.Sprintf("0x%02X", command),
		"dataHex":    fmt.Sprintf("%X", data),
		"packetHex":  fmt.Sprintf("%X", dnyPacket),
	}).Info("DNY命令发送成功")

	return nil
}

// fixDeviceGroupPhysicalID 修复设备组中Device的PhysicalID（私有，聚合到发送链路）
func (g *DeviceGateway) fixDeviceGroupPhysicalID(deviceID string, correctPhysicalID uint32) error {
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	// 通过设备索引找到ICCID和设备组
	iccidInterface, exists := g.tcpManager.GetDeviceIndex().Load(deviceID)
	if !exists {
		return fmt.Errorf("设备索引不存在")
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return fmt.Errorf("设备组不存在")
	}

	group := groupInterface.(*core.DeviceGroup)
	group.Lock()
	defer group.Unlock()

	// 修复Device的PhysicalID
	if device, ok := group.Devices[deviceID]; ok {
		device.Lock()
		device.PhysicalID = correctPhysicalID
		device.Unlock()
	}

	return nil
}
