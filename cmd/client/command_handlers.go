package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// handleRegisterResponse 处理注册响应
func (c *TestClient) handleRegisterResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 1 {
		response := result.Data[0]
		if response == 0x00 {
			c.logger.GetLogger().Info("✅ 设备注册成功")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"response": fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 设备注册失败")
		}
	}
}

// handleHeartbeatResponse 处理心跳响应
func (c *TestClient) handleHeartbeatResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 1 {
		response := result.Data[0]
		if response == 0x00 || response == 0x81 {
			c.logger.GetLogger().Debug("💓 心跳响应正常")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"response": fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 心跳响应异常")
		}
	}
}

// handleNetworkStatusQuery 处理网络状态查询（81指令）
func (c *TestClient) handleNetworkStatusQuery(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到网络状态查询指令，发送注册包和心跳包")

	// 发送注册包响应
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.SendRegister()
		time.Sleep(500 * time.Millisecond)
		c.SendHeartbeat()
	}()
}

// handleChargeControl 处理充电控制指令（82指令）
func (c *TestClient) handleChargeControl(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到充电控制指令，开始解析...")

	if len(result.Data) < 37 {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"dataLength": len(result.Data),
			"expected":   37,
		}).Error("❌ 充电控制指令数据长度不足")
		return
	}

	// 解析充电控制数据
	rateMode := result.Data[0]
	balance := binary.LittleEndian.Uint32(result.Data[1:5])
	portNumber := result.Data[5]
	chargeCommand := result.Data[6]
	chargeDuration := binary.LittleEndian.Uint16(result.Data[7:9])
	orderNumber := string(result.Data[9:25])

	c.logger.GetLogger().WithFields(logrus.Fields{
		"rateMode":       rateMode,
		"balance":        balance,
		"portNumber":     portNumber,
		"chargeCommand":  chargeCommand,
		"chargeDuration": chargeDuration,
		"orderNumber":    orderNumber,
	}).Info("📋 充电控制指令详情")

	// 发送充电控制响应
	c.sendChargeControlResponse(result.MessageID, portNumber, orderNumber)
}

// sendChargeControlResponse 发送充电控制响应
func (c *TestClient) sendChargeControlResponse(messageID uint16, portNumber uint8, orderNumber string) {
	c.logger.GetLogger().Info("📤 发送充电控制响应...")

	// 构建响应数据
	data := make([]byte, 20)

	// 应答（1字节）- 0=执行成功
	data[0] = 0x00

	// 订单编号（16字节）
	copy(data[1:17], []byte(orderNumber))

	// 端口号（1字节）
	data[17] = portNumber

	// 待充端口（2字节）- 0表示无待充端口
	binary.LittleEndian.PutUint16(data[18:20], 0)

	// 使用原消息ID发送响应
	packet := pkg.Protocol.BuildDNYResponsePacket(c.config.PhysicalID, messageID, dny_protocol.CmdChargeControl, data)

	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送充电控制响应失败")
		return
	}

	c.logger.GetLogger().Info("✅ 充电控制响应发送成功")
}

// handleSwipeCardResponse 处理刷卡响应
func (c *TestClient) handleSwipeCardResponse(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到刷卡响应")
	// 这里只是打印，实际应用中可能需要更多处理
}

// handleSettlementResponse 处理结算响应
func (c *TestClient) handleSettlementResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 21 { // 需要至少21字节：订单号(20) + 状态(1)
		// 提取订单号 (前20字节)
		orderNumber := string(bytes.TrimRight(result.Data[0:20], "\x00"))

		// 提取状态码 (第21字节)
		response := result.Data[20]

		if response == 0x00 {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"orderNumber": orderNumber,
			}).Info("✅ 结算信息上传成功")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"orderNumber": orderNumber,
				"response":    fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 结算信息上传失败")
		}
	} else {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"dataLen": len(result.Data),
		}).Error("❌ 结算响应数据长度不足")
	}
}
