package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// HandleServerMessages 处理服务器消息
func (c *TestClient) HandleServerMessages() {
	c.logger.GetLogger().Info("🎧 开始监听服务器消息...")

	buffer := make([]byte, 1024)

	for c.isRunning {
		// 设置读取超时
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		n, err := c.conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时继续循环
			}
			if c.isRunning {
				c.logger.GetLogger().WithError(err).Error("❌ 读取服务器消息失败")
			}
			break
		}

		if n > 0 {
			receivedData := buffer[:n]
			// 打印解析的数据
			c.logger.GetLogger().WithFields(logrus.Fields{
				"dataLen":    n,
				"dataHex":    hex.EncodeToString(receivedData),
				"dataStr":    string(receivedData),
				"remoteAddr": c.conn.RemoteAddr().String(),
				"localAddr":  c.conn.LocalAddr().String(),
				"timestamp":  time.Now().Format(time.RFC3339),
				"messageID":  c.getNextMessageID(),
				"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
			}).Info("📥 收到服务器数据")

			// 使用已有的解析函数
			if pkg.Protocol.IsDNYProtocolData(receivedData) {
				c.handleDNYMessage(receivedData)
			} else {
				c.logger.GetLogger().WithFields(logrus.Fields{
					"dataStr": string(receivedData),
				}).Info("📥 收到非DNY协议数据")
			}
		}
	}
}

// handleDNYMessage 处理DNY协议消息
func (c *TestClient) handleDNYMessage(data []byte) {
	// 使用已有的解析函数
	result, err := pkg.Protocol.ParseDNYData(data)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 解析DNY消息失败")
		return
	}

	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":     fmt.Sprintf("0x%02X", result.Command),
		"commandName": result.CommandName,
		"physicalID":  fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":   result.MessageID,
		"dataLen":     len(result.Data),
		"checksumOK":  result.ChecksumValid,
	}).Info("📋 解析DNY消息")

	// 根据命令类型进行处理
	switch result.Command {
	case dny_protocol.CmdDeviceRegister:
		c.handleRegisterResponse(result)
	case dny_protocol.CmdMainHeartbeat:
		c.handleMainHeartbeatResponse(result)
	case dny_protocol.CmdGetServerTime:
		c.handleServerTimeResponse(result)
	case dny_protocol.CmdMainStatusReport:
		c.handleMainStatusResponse(result)
	case dny_protocol.CmdDeviceHeart:
		c.handleHeartbeatResponse(result)
	case dny_protocol.CmdNetworkStatus:
		c.handleNetworkStatusQuery(result)
	case dny_protocol.CmdChargeControl:
		c.handleChargeControl(result)
	case dny_protocol.CmdSwipeCard:
		c.handleSwipeCardResponse(result)
	case dny_protocol.CmdSettlement:
		c.handleSettlementResponse(result)
	case dny_protocol.CmdUpgradeNew:
		c.handleFirmwareUpgrade(result)
	default:
		c.logger.GetLogger().WithFields(logrus.Fields{
			"command": fmt.Sprintf("0x%02X", result.Command),
		}).Info("📋 收到未处理的指令，仅打印信息")
	}
}

// SendRegister 发送设备注册包（20指令）- 匹配真实设备格式
func (c *TestClient) SendRegister() error {
	c.logger.GetLogger().Info("📤 发送设备注册包（0x20指令）...")

	// 构建注册包数据 - 根据线上数据调整为6字节格式
	// 线上数据示例：8002021e3106 (固件版本=640, 端口数=2, 虚拟ID=30, 设备类型=49, 工作模式=6)
	data := make([]byte, 6)

	// 固件版本（2字节，线上显示为0x8002，表示版本640）
	data[0] = 0x80
	data[1] = 0x02

	// 端口数量（1字节）
	data[2] = c.config.PortCount

	// 虚拟ID（1字节）- 使用配置中的虚拟ID或根据物理ID生成
	if c.config.VirtualID > 0 {
		data[3] = c.config.VirtualID
	} else {
		data[3] = byte(c.config.PhysicalID & 0xFF) // 使用物理ID的低8位作为虚拟ID
	}

	// 设备类型（1字节）- 线上数据显示为0x31（49）
	data[4] = 0x31

	// 工作模式（1字节）- 线上数据显示为0x06
	data[5] = 0x06

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdDeviceRegister, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":  fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"deviceType":  fmt.Sprintf("0x%02X", c.config.DeviceType),
		"firmwareVer": c.config.FirmwareVer,
		"portCount":   c.config.PortCount,
		"packetHex":   hex.EncodeToString(packet),
		"packetLen":   len(packet),
	}).Info("📦 注册包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送注册包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 注册包发送成功")
	return nil
}

// SendMainHeartbeat 发送主机状态心跳包（0x11指令）- 每30分钟发送一次
func (c *TestClient) SendMainHeartbeat() error {
	c.logger.GetLogger().Info("💓 发送主机状态心跳包（0x11指令）...")

	// 构建主机心跳数据 - 按照协议文档：
	// 固件版本(2) + RTC模块(1) + 时间戳(4) + 信号强度(1) + 通讯模块类型(1) + SIM卡号(20) + 主机类型(1) + 频率(2) + IMEI(15) + 模块版本号(24)
	data := make([]byte, 71)
	offset := 0

	// 固件版本（2字节，小端序）
	binary.LittleEndian.PutUint16(data[offset:offset+2], c.config.FirmwareVer)
	offset += 2

	// RTC模块类型（1字节）
	data[offset] = c.config.RTCType
	offset += 1

	// 主机当前时间戳（4字节，小端序）- 如无RTC模块则为全0
	if c.config.HasRTC {
		binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(time.Now().Unix()))
	} else {
		binary.LittleEndian.PutUint32(data[offset:offset+4], 0)
	}
	offset += 4

	// 信号强度（1字节）
	data[offset] = c.config.SignalStrength
	offset += 1

	// 通讯模块类型（1字节）
	data[offset] = c.config.CommType
	offset += 1

	// SIM卡号（20字节）- ICCID
	iccidBytes := []byte(c.config.ICCID)
	if len(iccidBytes) > 20 {
		copy(data[offset:offset+20], iccidBytes[:20])
	} else {
		copy(data[offset:offset+len(iccidBytes)], iccidBytes)
	}
	offset += 20

	// 主机类型（1字节）
	data[offset] = c.config.HostType
	offset += 1

	// 频率（2字节，小端序）- LORA使用的中心频率，如无此数据则为0
	binary.LittleEndian.PutUint16(data[offset:offset+2], c.config.Frequency)
	offset += 2

	// IMEI号（15字节）
	imeiBytes := []byte(c.config.IMEI)
	if len(imeiBytes) > 15 {
		copy(data[offset:offset+15], imeiBytes[:15])
	} else {
		copy(data[offset:offset+len(imeiBytes)], imeiBytes)
	}
	offset += 15

	// 模块版本号（24字节）
	moduleVerBytes := []byte(c.config.ModuleVersion)
	if len(moduleVerBytes) > 24 {
		copy(data[offset:offset+24], moduleVerBytes[:24])
	} else {
		copy(data[offset:offset+len(moduleVerBytes)], moduleVerBytes)
	}

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdMainHeartbeat, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":     fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"firmwareVer":    c.config.FirmwareVer,
		"rtcType":        fmt.Sprintf("0x%02X", c.config.RTCType),
		"signalStrength": c.config.SignalStrength,
		"commType":       fmt.Sprintf("0x%02X", c.config.CommType),
		"hostType":       fmt.Sprintf("0x%02X", c.config.HostType),
		"frequency":      c.config.Frequency,
		"imei":           c.config.IMEI,
		"moduleVersion":  c.config.ModuleVersion,
		"packetHex":      hex.EncodeToString(packet),
		"packetLen":      len(packet),
	}).Info("📦 主机心跳包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送主机心跳包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 主机心跳包发送成功")
	return nil
}

// SendGetServerTime 发送获取服务器时间请求（0x12指令）
func (c *TestClient) SendGetServerTime() error {
	c.logger.GetLogger().Info("🕐 发送获取服务器时间请求（0x12指令）...")

	// 无数据，只发送命令
	data := make([]byte, 0)

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdGetServerTime, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"packetHex":  hex.EncodeToString(packet),
		"packetLen":  len(packet),
	}).Info("📦 获取服务器时间请求包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送获取服务器时间请求失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 获取服务器时间请求发送成功")
	return nil
}

// SendMainStatusReport 发送主机状态包上报（0x17指令）- 每30分钟发送一次
func (c *TestClient) SendMainStatusReport() error {
	c.logger.GetLogger().Info("📊 发送主机状态包上报（0x17指令）...")

	// 构建状态包数据 - 根据实际需要调整数据结构
	data := make([]byte, 8)

	// 主机工作状态（1字节）- 0x00=正常
	data[0] = 0x00

	// 电压（2字节，小端序）- 模拟220V
	binary.LittleEndian.PutUint16(data[1:3], 2200) // 220.0V

	// 当前环境温度（1字节）- 模拟25度，需要加65
	data[3] = 65 + 25

	// 端口数量（1字节）
	data[4] = c.config.PortCount

	// 各端口状态（n字节，这里简化为2字节）- 0=空闲
	data[5] = 0x00 // 端口1状态
	data[6] = 0x00 // 端口2状态

	// 预留字节
	data[7] = 0x00

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdMainStatusReport, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":  fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"voltage":     "220.0V",
		"temperature": "25°C",
		"portCount":   c.config.PortCount,
		"packetHex":   hex.EncodeToString(packet),
		"packetLen":   len(packet),
	}).Info("📦 主机状态包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送主机状态包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 主机状态包发送成功")
	return nil
}

// handleMainHeartbeatResponse 处理主机心跳响应
func (c *TestClient) handleMainHeartbeatResponse(result *protocol.DNYParseResult) {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  result.MessageID,
		"dataLen":    len(result.Data),
		"dataHex":    hex.EncodeToString(result.Data),
	}).Info("📥 收到主机心跳响应")

	// 根据协议文档，主机心跳(0x11)服务器应答：无须应答
	// 如果收到数据，说明可能是其他设备发送的心跳数据，记录但不解析响应码
	if len(result.Data) > 0 {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"note": "协议规定服务器无须应答主机心跳，此数据可能来自其他设备",
		}).Info("📋 主机心跳包含数据")
	} else {
		c.logger.GetLogger().Info("✅ 主机心跳确认成功（无数据，符合协议规范）")
	}
}

// handleServerTimeResponse 处理服务器时间响应
func (c *TestClient) handleServerTimeResponse(result *protocol.DNYParseResult) {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  result.MessageID,
		"dataLen":    len(result.Data),
		"dataHex":    hex.EncodeToString(result.Data),
		"dataStr":    string(result.Data),
		// 原始数据
		"rawDataHex": hex.EncodeToString(result.RawData),
	}).Info("📥 收到服务器时间响应")

	if len(result.Data) >= 4 {
		// 根据协议文档，服务器时间响应格式：时间戳(4字节)，无应答码
		// 协议规定：命令 + 时间戳(4字节)，这里的 result.Data 只包含时间戳部分
		timestamp := binary.LittleEndian.Uint32(result.Data[0:4])
		serverTime := time.Unix(int64(timestamp), 0)

		c.logger.GetLogger().WithFields(logrus.Fields{
			"serverTime":      serverTime.Format(constants.TimeFormatDefault),
			"serverTimestamp": timestamp,
			"localTime":       time.Now().Format(constants.TimeFormatDefault),
		}).Info("🕐 服务器时间获取成功")

		// 实现时间同步逻辑
		timeDiff := time.Now().Unix() - int64(timestamp)
		if abs(timeDiff) > 60 { // 如果时间差超过1分钟
			c.logger.GetLogger().WithFields(logrus.Fields{
				"timeDifference": fmt.Sprintf("%d秒", timeDiff),
			}).Warn("⚠️ 本地时间与服务器时间差异较大")
		}
	} else {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"expectedLength": 4,
			"actualLength":   len(result.Data),
		}).Error("❌ 服务器时间响应数据长度不足，应为4字节时间戳")
	}
}

// handleMainStatusResponse 处理主机状态包响应
func (c *TestClient) handleMainStatusResponse(result *protocol.DNYParseResult) {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  result.MessageID,
		"dataLen":    len(result.Data),
		"dataHex":    hex.EncodeToString(result.Data),
	}).Info("📥 收到主机状态包响应")

	// 根据协议文档，主机状态包(0x17)服务器无需应答
	// 如果收到数据，说明可能是其他设备发送的状态数据，记录但不解析响应码
	if len(result.Data) > 0 {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"note": "协议规定服务器无需应答主机状态包，此数据可能来自其他设备",
		}).Info("📋 主机状态包含数据")
	} else {
		c.logger.GetLogger().Info("✅ 主机状态包确认成功（无数据，符合协议规范）")
	}
}

// handleFirmwareUpgrade 处理固件升级指令
func (c *TestClient) handleFirmwareUpgrade(result *protocol.DNYParseResult) {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  result.MessageID,
		"dataLen":    len(result.Data),
	}).Info("📥 收到固件升级指令")

	// 根据不同的升级命令处理
	switch result.Command {
	case dny_protocol.CmdUpgradeNew: // 0xFA - 主机固件升级（新版）
		c.handleNewFirmwareUpgrade(result)
	case dny_protocol.CmdUpgradeOld: // 0xF8 - 设备固件升级（旧版）
		c.handleOldFirmwareUpgrade(result)
	default:
		c.logger.GetLogger().WithFields(logrus.Fields{
			"command": fmt.Sprintf("0x%02X", result.Command),
		}).Warn("⚠️ 未知的固件升级命令")
	}
}

// handleNewFirmwareUpgrade 处理新版固件升级（0xFA）
func (c *TestClient) handleNewFirmwareUpgrade(result *protocol.DNYParseResult) {
	if len(result.Data) == 0 {
		// 触发升级模式指令
		c.logger.GetLogger().Info("🔄 收到触发固件升级模式指令")

		// 发送设备请求固件升级响应
		responseData := make([]byte, 3)
		responseData[0] = 0x00                                   // 应答：0=成功
		binary.LittleEndian.PutUint16(responseData[1:3], 0x0000) // 请求升级固定为0000

		packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdUpgradeNew, responseData)
		c.conn.Write(packet)

		c.logger.GetLogger().Info("✅ 已发送设备请求固件升级响应")
	} else if len(result.Data) >= 4 {
		// 固件数据包
		totalPackets := binary.LittleEndian.Uint16(result.Data[0:2])
		currentPacket := binary.LittleEndian.Uint16(result.Data[2:4])
		firmwareData := result.Data[4:]

		c.logger.GetLogger().WithFields(logrus.Fields{
			"totalPackets":  totalPackets,
			"currentPacket": currentPacket,
			"firmwareSize":  len(firmwareData),
		}).Info("📦 收到固件数据包")

		// 模拟固件包处理成功
		responseData := make([]byte, 3)
		responseData[0] = 0x00 // 应答：0=成功，可以发送下一包
		binary.LittleEndian.PutUint16(responseData[1:3], currentPacket)

		packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdUpgradeNew, responseData)
		c.conn.Write(packet)

		if currentPacket == totalPackets {
			c.logger.GetLogger().Info("🎉 固件升级完成")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"progress": fmt.Sprintf("%d/%d", currentPacket, totalPackets),
			}).Info("⏳ 固件升级进度")
		}
	}
}

// handleOldFirmwareUpgrade 处理旧版固件升级（0xF8）
func (c *TestClient) handleOldFirmwareUpgrade(result *protocol.DNYParseResult) {
	if len(result.Data) >= 4 {
		totalPackets := binary.LittleEndian.Uint16(result.Data[0:2])
		currentPacket := binary.LittleEndian.Uint16(result.Data[2:4])
		firmwareData := result.Data[4:]

		c.logger.GetLogger().WithFields(logrus.Fields{
			"totalPackets":  totalPackets,
			"currentPacket": currentPacket,
			"firmwareSize":  len(firmwareData),
		}).Info("📦 收到旧版固件数据包")

		// 模拟固件包处理成功
		responseData := make([]byte, 3)
		responseData[0] = 0x00 // 应答：0=成功，可以发送下一包
		binary.LittleEndian.PutUint16(responseData[1:3], currentPacket)

		packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdUpgradeOld, responseData)
		c.conn.Write(packet)

		if currentPacket == totalPackets {
			c.logger.GetLogger().Info("🎉 旧版固件升级完成")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"progress": fmt.Sprintf("%d/%d", currentPacket, totalPackets),
			}).Info("⏳ 旧版固件升级进度")
		}
	}
}

// SendDeviceHeartbeat01 发送设备心跳（0x01指令）- 模拟线上真实数据
func (c *TestClient) SendDeviceHeartbeat01() error {
	c.logger.GetLogger().Info("💓 发送设备心跳包（0x01指令）...")

	// 构建心跳数据 - 根据线上数据：8002e80802000000000000000000000a00316100（20字节）
	data := make([]byte, 20)

	// 固件版本（2字节）
	data[0] = 0x80
	data[1] = 0x02

	// 时间戳或状态标识（4字节）
	data[2] = 0xe8
	data[3] = 0x08
	data[4] = 0x02
	data[5] = 0x00

	// 预留字段（10字节全零）
	for i := 6; i < 16; i++ {
		data[i] = 0x00
	}

	// 状态信息（4字节）
	data[16] = 0x0a
	data[17] = 0x00
	data[18] = 0x31
	data[19] = 0x61

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), 0x01, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"packetHex":  hex.EncodeToString(packet),
		"packetLen":  len(packet),
		"dataHex":    hex.EncodeToString(data),
	}).Info("📦 设备心跳包（0x01）详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送设备心跳包（0x01）失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 设备心跳包（0x01）发送成功")
	return nil
}

// SendDeviceHeartbeat21 发送设备心跳（0x21指令）- 模拟线上真实数据
func (c *TestClient) SendDeviceHeartbeat21() error {
	c.logger.GetLogger().Info("💓 发送设备心跳包（0x21指令）...")

	// 构建心跳数据 - 根据线上数据：e8080200000061（7字节）
	data := make([]byte, 7)

	data[0] = 0xe8
	data[1] = 0x08
	data[2] = 0x02
	data[3] = 0x00
	data[4] = 0x00
	data[5] = 0x00
	data[6] = 0x61

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), 0x21, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"packetHex":  hex.EncodeToString(packet),
		"packetLen":  len(packet),
		"dataHex":    hex.EncodeToString(data),
	}).Info("📦 设备心跳包（0x21）详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送设备心跳包（0x21）失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 设备心跳包（0x21）发送成功")
	return nil
}

// SendLinkHeartbeat 发送"link"字符串心跳 - 模拟线上真实数据
func (c *TestClient) SendLinkHeartbeat() error {
	c.logger.GetLogger().Info("💓 发送link字符串心跳...")

	// 直接发送"link"字符串
	linkData := []byte("link")

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"dataStr":    string(linkData),
		"dataHex":    hex.EncodeToString(linkData),
		"dataLen":    len(linkData),
	}).Info("📦 link心跳详情")

	// 发送数据包
	_, err := c.conn.Write(linkData)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送link心跳失败")
		return err
	}

	c.logger.GetLogger().Info("✅ link心跳发送成功")
	return nil
}

// SendServerTimeRequest 发送服务器时间请求（0x22指令）- 模拟线上真实数据
func (c *TestClient) SendServerTimeRequest() error {
	c.logger.GetLogger().Info("🕐 发送服务器时间请求（0x22指令）...")

	// 无数据，只发送命令
	data := make([]byte, 0)

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), 0x22, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"packetHex":  hex.EncodeToString(packet),
		"packetLen":  len(packet),
	}).Info("📦 服务器时间请求包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送服务器时间请求失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 服务器时间请求发送成功")
	return nil
}

// =====================================================================
// 向后兼容性方法 - 保持测试序列正常运行
// =====================================================================

// SendSwipeCard 发送刷卡请求（向后兼容方法）
func (c *TestClient) SendSwipeCard(cardID uint32, portNumber uint8) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"cardID":     fmt.Sprintf("0x%08X", cardID),
		"portNumber": portNumber,
	}).Info("💳 发送刷卡请求...")

	// 构建刷卡数据包
	data := make([]byte, 13) // 基础长度：4+1+1+2+4+1 = 13字节
	offset := 0

	// 卡片ID（4字节，小端序）
	binary.LittleEndian.PutUint32(data[offset:offset+4], cardID)
	offset += 4

	// 卡片类型（1字节）- 默认为新卡
	data[offset] = 1
	offset += 1

	// 端口号（1字节）
	data[offset] = portNumber
	offset += 1

	// 余额卡内金额（2字节，小端序）- 默认5000分
	binary.LittleEndian.PutUint16(data[offset:offset+2], 5000)
	offset += 2

	// 时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(time.Now().Unix()))
	offset += 4

	// 卡号2字节数（1字节）- 无额外卡号
	data[offset] = 0

	// 构建DNY包
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdSwipeCard, data)

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送刷卡请求失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 刷卡请求发送成功")
	return nil
}

// SendSettlement 发送结算数据（向后兼容方法）
func (c *TestClient) SendSettlement(gunNumber uint8, chargeDuration uint32, electricEnergy uint32, totalFee uint32) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"gunNumber":      gunNumber,
		"chargeDuration": chargeDuration,
		"electricEnergy": electricEnergy,
		"totalFee":       totalFee,
	}).Info("💰 发送结算数据...")

	now := time.Now()
	startTime := now.Add(-time.Duration(chargeDuration) * time.Second)

	// 构建结算数据包 - 总长度：20+20+4+4+4+4+4+4+1+1 = 66字节
	data := make([]byte, 66)
	offset := 0

	// 订单号（20字节）
	orderID := fmt.Sprintf("ORDER%d", now.Unix())
	copy(data[offset:offset+20], []byte(orderID))
	offset += 20

	// 卡号（20字节）
	cardNumber := "1234567890123456"
	copy(data[offset:offset+20], []byte(cardNumber))
	offset += 20

	// 开始时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(startTime.Unix()))
	offset += 4

	// 结束时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(now.Unix()))
	offset += 4

	// 充电电量（4字节，小端序）- Wh
	binary.LittleEndian.PutUint32(data[offset:offset+4], electricEnergy)
	offset += 4

	// 充电费用（4字节，小端序）- 分
	chargeFee := totalFee * 80 / 100 // 假设充电费用占总费用的80%
	binary.LittleEndian.PutUint32(data[offset:offset+4], chargeFee)
	offset += 4

	// 服务费（4字节，小端序）- 分
	serviceFee := totalFee - chargeFee
	binary.LittleEndian.PutUint32(data[offset:offset+4], serviceFee)
	offset += 4

	// 总费用（4字节，小端序）- 分
	binary.LittleEndian.PutUint32(data[offset:offset+4], totalFee)
	offset += 4

	// 枪号（1字节）
	data[offset] = gunNumber
	offset += 1

	// 停止原因（1字节）- 0表示正常停止
	data[offset] = 0

	// 构建DNY包
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdSettlement, data)

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送结算数据失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 结算数据发送成功")
	return nil
}

// SendHeartbeat 发送普通设备心跳（向后兼容方法）
func (c *TestClient) SendHeartbeat() error {
	c.logger.GetLogger().Info("💓 发送设备心跳包（向后兼容）...")

	// 构建心跳数据包 - 简单的2字节数据
	data := make([]byte, 2)
	data[0] = 0x01 // 心跳类型
	data[1] = 0x00 // 设备状态：正常

	// 构建DNY包
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdDeviceHeart, data)

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送设备心跳失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 设备心跳发送成功")
	return nil
}

// abs 计算绝对值
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
