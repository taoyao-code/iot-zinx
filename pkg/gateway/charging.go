package gateway

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

const (
	actionDescStop  = "停止充电"
	actionDescStart = "开始充电"
)

// SendChargingCommand 发送充电控制命令（简版）
func (g *DeviceGateway) SendChargingCommand(deviceID string, port uint8, action uint8) error {
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
	}

	// 🔧 修复CVE-Critical-003: 统一端口转换策略
	// 协议要求使用0-based端口号，外部传入1-based，需要转换为port-1
	protocolPort := port - 1
	commandData := []byte{protocolPort, action}

	actionStr := "STOP_CHARGING"
	actionDesc := actionDescStop
	if action == 0x01 {
		actionStr = "START_CHARGING"
		actionDesc = actionDescStart
	}

	if err := g.SendCommandToDevice(deviceID, constants.CmdChargeControl, commandData); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":     deviceID,
			"command":      "CHARGE_CONTROL",
			"commandID":    fmt.Sprintf("0x%02X", constants.CmdChargeControl),
			"port":         port,
			"protocolPort": protocolPort,
			"action":       actionStr,
			"actionCode":   fmt.Sprintf("0x%02X", action),
			"error":        err.Error(),
			"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
		}).Error("❌ 充电控制命令发送失败")
		return fmt.Errorf("发送充电控制命令失败: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"command":      "CHARGE_CONTROL",
		"commandID":    fmt.Sprintf("0x%02X", constants.CmdChargeControl),
		"port":         port,
		"protocolPort": protocolPort,
		"action":       actionStr,
		"actionCode":   fmt.Sprintf("0x%02X", action),
		"actionDesc":   actionDesc,
		"status":       "SENT",
		"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
		"dataLen":      len(commandData),
	}).Info("⚡ 充电控制命令发送成功")

	return nil
}

// SendChargingCommandWithParams 发送完整参数的充电控制命令（0x82）
func (g *DeviceGateway) SendChargingCommandWithParams(deviceID string, port uint8, action uint8, orderNo string, mode uint8, value uint16, balance uint32) error {
	if deviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
	}
	if len(orderNo) > 16 {
		return fmt.Errorf("订单号长度超过限制：当前%d字节，最大16字节，订单号：%s", len(orderNo), orderNo)
	}
	if action > 1 {
		return fmt.Errorf("充电动作无效：%d，有效值：0(停止)或1(开始)", action)
	}
	if action == 0x01 {
		if mode > 1 {
			return fmt.Errorf("充电模式无效：%d，有效值：0(按时间)或1(按电量)", mode)
		}
		if mode == 0 && value == 0 {
			return fmt.Errorf("按时间充电时，充电时长不能为0秒")
		}
		if mode == 1 && value == 0 {
			return fmt.Errorf("按电量充电时，充电电量不能为0")
		}
		if balance == 0 {
			return fmt.Errorf("余额不能为0")
		}
		if value == 0 {
			return fmt.Errorf("充电值不能为0")
		}
	}

	commandData := make([]byte, 37)
	commandData[0] = mode
	commandData[1] = byte(balance)
	commandData[2] = byte(balance >> 8)
	commandData[3] = byte(balance >> 16)
	commandData[4] = byte(balance >> 24)
	commandData[5] = port - 1
	commandData[6] = action
	actualValue := value
	commandData[7] = byte(actualValue)
	commandData[8] = byte(actualValue >> 8)
	orderBytes := make([]byte, 16)
	if len(orderNo) > 0 {
		copy(orderBytes, []byte(orderNo))
	}
	copy(commandData[9:25], orderBytes)

	var maxChargeDuration uint16
	if action == 0x01 {
		if mode == 0 && actualValue > 0 {
			maxChargeDuration = actualValue + (actualValue / 2)
			if maxChargeDuration > 36000 {
				maxChargeDuration = 36000
			}
		} else {
			maxChargeDuration = 0
		}
	} else {
		maxChargeDuration = 0
	}
	commandData[25] = byte(maxChargeDuration)
	commandData[26] = byte(maxChargeDuration >> 8)

	overloadPower := uint16(0)
	commandData[27] = byte(overloadPower)
	commandData[28] = byte(overloadPower >> 8)
	commandData[29] = 0 // 二维码灯：0=打开
	commandData[30] = 0 // 长充模式：0=关闭
	commandData[31] = 0 // 额外浮充时间
	commandData[32] = 0
	commandData[33] = 2 // 是否跳过短路检测=2正常
	commandData[34] = 0 // 不判断用户拔出
	commandData[35] = 0 // 强制带充满自停
	commandData[36] = 0 // 充满功率(单位1W)，此处关闭

	if err := g.SendCommandToDevice(deviceID, constants.CmdChargeControl, commandData); err != nil {
		return fmt.Errorf("发送充电控制命令失败: %v", err)
	}

	actionStr := actionDescStop
	if action == 0x01 {
		actionStr = actionDescStart
	}
	modeStr := "按时间"
	if mode == 1 {
		modeStr = "按电量"
	}
	logger.WithFields(logrus.Fields{
		"deviceID":          deviceID,
		"port":              port,
		"action":            actionStr,
		"orderNo":           orderNo,
		"mode":              modeStr,
		"value":             actualValue,
		"maxChargeDuration": maxChargeDuration,
		"balance":           balance,
		"unit":              getValueUnit(mode),
	}).Info("🔧 修复最大充电时长后的完整参数充电控制命令发送成功")

	// 🔧 修复CVE-Critical-001: 使用订单管理器替换简单的OrderContext
	if action == 0x01 && orderNo != "" {
		// 创建订单记录到订单管理器
		if err := g.orderManager.CreateOrder(deviceID, int(port), orderNo, mode, actualValue, balance); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"port":     port,
				"orderNo":  orderNo,
				"error":    err.Error(),
			}).Warn("订单管理器创建订单失败，但充电命令已发送")
			// 不返回错误，因为充电命令已经发送成功
		} else {
			// 订单创建成功，更新状态为充电中
			g.orderManager.UpdateOrderStatus(deviceID, int(port), OrderStatusCharging, "充电命令发送成功")
		}
	}

	return nil
}

// SendStopChargingCommand 发送停止充电命令
func (g *DeviceGateway) SendStopChargingCommand(deviceID string, port uint8, orderNo string) error {
	return g.SendChargingCommandWithParams(deviceID, port, 0x00, orderNo, 0, 0, 0)
}

// UpdateChargingOverloadPower 仅更新过载功率/最大充电时长
func (g *DeviceGateway) UpdateChargingOverloadPower(deviceID string, port uint8, orderNo string, overloadPowerW uint16, maxChargeDurationSeconds uint16) error {
	if deviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
	}
	if len(orderNo) > 16 {
		return fmt.Errorf("订单号长度超过限制：%d", len(orderNo))
	}
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}
	if !g.IsDeviceOnline(deviceID) {
		return fmt.Errorf("设备不在线")
	}

	// 🔧 修复CVE-Critical-001: 使用订单管理器获取订单信息
	mode := uint8(0)
	value := uint16(0)
	balance := uint32(0)
	if orderNo != "" {
		if order := g.orderManager.GetOrder(deviceID, int(port)); order != nil && order.OrderNo == orderNo {
			mode = order.Mode
			value = order.Value
			balance = order.Balance
		} else if order != nil {
			return fmt.Errorf("订单号不匹配，当前订单: %s，请求更新订单: %s", order.OrderNo, orderNo)
		} else {
			return fmt.Errorf("未找到端口 %s:%d 上的进行中订单", deviceID, port)
		}
	}

	payload := make([]byte, 37)
	payload[0] = mode
	payload[1] = byte(balance)
	payload[2] = byte(balance >> 8)
	payload[3] = byte(balance >> 16)
	payload[4] = byte(balance >> 24)
	payload[5] = port - 1
	payload[6] = 0x01 // 保持充电
	payload[7] = byte(value)
	payload[8] = byte(value >> 8)
	orderBytes := make([]byte, 16)
	copy(orderBytes, []byte(orderNo))
	copy(payload[9:25], orderBytes)
	payload[25] = byte(maxChargeDurationSeconds)
	payload[26] = byte(maxChargeDurationSeconds >> 8)
	payload[27] = byte(overloadPowerW)
	payload[28] = byte(overloadPowerW >> 8)
	payload[29] = 0
	payload[30] = 0
	payload[31], payload[32] = 0, 0
	payload[33] = 2
	payload[34] = 0
	payload[35] = 0
	payload[36] = 0

	if err := g.SendCommandToDevice(deviceID, constants.CmdChargeControl, payload); err != nil {
		return err
	}

	logger.WithFields(logrus.Fields{
		"deviceID":                 deviceID,
		"port":                     port,
		"orderNo":                  orderNo,
		"overloadPowerW":           overloadPowerW,
		"maxChargeDurationSeconds": maxChargeDurationSeconds,
		"ctxMode":                  mode,
		"ctxValue":                 value,
		"ctxBalance":               balance,
	}).Info("已下发0x82仅更新过载功率/最大时长")

	return nil
}

func getValueUnit(mode uint8) string {
	if mode == 0 {
		return "秒"
	}
	return "0.1度"
}
