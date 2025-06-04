package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg"
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
	default:
		c.logger.GetLogger().WithFields(logrus.Fields{
			"command": fmt.Sprintf("0x%02X", result.Command),
		}).Info("📋 收到未处理的指令，仅打印信息")
	}
}

// SendRegister 发送设备注册包（20指令）
func (c *TestClient) SendRegister() error {
	c.logger.GetLogger().Info("📤 发送设备注册包（0x20指令）...")

	// 构建注册包数据
	data := make([]byte, 8)

	// 固件版本（2字节，小端序）
	binary.LittleEndian.PutUint16(data[0:2], c.config.FirmwareVer)

	// 端口数量（1字节）
	data[2] = c.config.PortCount

	// 虚拟ID（1字节）- 不需组网设备默认为00
	data[3] = 0x00

	// 设备类型（1字节）
	data[4] = c.config.DeviceType

	// 工作模式（1字节）- 第0位：0=联网，其他位保留
	data[5] = 0x00

	// 电源板版本号（2字节）- 无电源板为0
	binary.LittleEndian.PutUint16(data[6:8], 0)

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

// SendHeartbeat 发送心跳包（21指令）
func (c *TestClient) SendHeartbeat() error {
	c.logger.GetLogger().Debug("💓 发送心跳包（0x21指令）...")

	// 构建心跳包数据
	data := make([]byte, 5+c.config.PortCount)

	// 电压（2字节，小端序）- 模拟220V
	binary.LittleEndian.PutUint16(data[0:2], 2200) // 220.0V

	// 端口数量（1字节）
	data[2] = c.config.PortCount

	// 各端口状态（n字节）- 0=空闲
	for i := uint8(0); i < c.config.PortCount; i++ {
		data[3+i] = 0x00 // 空闲状态
	}

	// 信号强度（1字节）- 有线组网为00
	data[3+c.config.PortCount] = 0x00

	// 当前环境温度（1字节）- 模拟25度，需要加65
	data[4+c.config.PortCount] = 65 + 25

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdDeviceHeart, data)

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送心跳包失败")
		return err
	}

	c.logger.GetLogger().WithFields(logrus.Fields{
		"voltage":     "220.0V",
		"portCount":   c.config.PortCount,
		"temperature": "25°C",
	}).Debug("✅ 心跳包发送成功")

	return nil
}

// SendSwipeCard 发送刷卡操作（02指令）
func (c *TestClient) SendSwipeCard(cardID uint32, portNumber uint8) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"cardID":     fmt.Sprintf("0x%08X", cardID),
		"portNumber": portNumber,
	}).Info("📤 发送刷卡操作（0x02指令）...")

	// 构建刷卡数据
	data := make([]byte, 13)

	// 卡片ID（4字节，小端序）
	binary.LittleEndian.PutUint32(data[0:4], cardID)

	// 卡片类型（1字节）- 0=旧卡
	data[4] = 0x00

	// 端口号（1字节）
	data[5] = portNumber

	// 余额卡内金额（2字节，小端序）- 0表示非余额卡
	binary.LittleEndian.PutUint16(data[6:8], 0)

	// 时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[8:12], uint32(time.Now().Unix()))

	// 卡号2字节数（1字节）- 0表示无额外卡号
	data[12] = 0x00

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdSwipeCard, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"packetHex": hex.EncodeToString(packet),
		"packetLen": len(packet),
	}).Info("📦 刷卡包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送刷卡包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 刷卡包发送成功")
	return nil
}

// SendSettlement 发送结算信息（03指令）
func (c *TestClient) SendSettlement(portNumber uint8, chargeDuration uint16, maxPower uint16, energyConsumed uint16) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"portNumber":     portNumber,
		"chargeDuration": chargeDuration,
		"maxPower":       maxPower,
		"energyConsumed": energyConsumed,
	}).Info("📤 发送结算信息（0x03指令）...")

	// 构建结算数据（35字节）
	data := make([]byte, 35)

	// 充电时长（2字节，小端序）
	binary.LittleEndian.PutUint16(data[0:2], chargeDuration)

	// 最大功率（2字节，小端序）
	binary.LittleEndian.PutUint16(data[2:4], maxPower)

	// 耗电量（2字节，小端序）
	binary.LittleEndian.PutUint16(data[4:6], energyConsumed)

	// 端口号（1字节）
	data[6] = portNumber

	// 在线/离线启动（1字节）- 1=在线启动
	data[7] = 0x01

	// 卡号/验证码（4字节）- 在线启动时为全0
	binary.LittleEndian.PutUint32(data[8:12], 0)

	// 停止原因（1字节）- 1=充满自停
	data[12] = 0x01

	// 订单编号（16字节）- 模拟订单号
	orderNumber := "TEST_ORDER_001234"
	copy(data[13:29], []byte(orderNumber))

	// 第二最大功率（2字节，小端序）
	binary.LittleEndian.PutUint16(data[29:31], maxPower)

	// 时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[31:35], uint32(time.Now().Unix()))

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, c.getNextMessageID(), dny_protocol.CmdSettlement, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"packetHex": hex.EncodeToString(packet),
		"packetLen": len(packet),
	}).Info("📦 结算包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送结算包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 结算包发送成功")
	return nil
}
