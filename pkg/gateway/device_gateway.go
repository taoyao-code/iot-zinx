/*
 * @Author: IoT-Zinx团队
 * @Date: 2025-08-08 16:00:00
 * @LastEditors: IoT-Zinx团队
 * @LastEditTime: 2025-08-08 16:00:00
 * @Description: 设备网关统一接口层
 *
 * 【重要！！！重要！！！重要！！！】
 * 这里是IoT设备网关的核心组件库！
 * 借鉴WebSocket网关的简洁设计理念，提供统一的设备管理接口，除非你知道这意味着什么！
 */

package gateway

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

/**
 *  【注意！！！注意！！！注意！！！】
 *  这里是IoT设备网关的核心组件库！
 *  基于WebSocket网关的简洁设计理念
 *  为IoT设备管理提供统一的对外接口
 *  请谨慎修改此处的代码，除非你知道这意味着什么！
 */

// DeviceGateway IoT设备网关统一接口
// 提供简洁、直观的设备管理API，隐藏底层复杂实现
type DeviceGateway struct {
	tcpManager *core.TCPManager
	tcpWriter  *network.TCPWriter // 🚀 Phase 2: 添加TCPWriter支持重试机制
}

// NewDeviceGateway 创建设备网关实例
func NewDeviceGateway() *DeviceGateway {
	// 🔧 修复：从配置创建TCPWriter，设置正确的写超时时间
	retryConfig := network.DefaultRetryConfig

	// 尝试从全局配置获取TCP写超时配置
	if globalConfig := config.GetConfig(); globalConfig != nil {
		if globalConfig.TCPServer.TCPWriteTimeoutSeconds > 0 {
			retryConfig.WriteTimeout = time.Duration(globalConfig.TCPServer.TCPWriteTimeoutSeconds) * time.Second
			logger.GetLogger().WithFields(logrus.Fields{
				"writeTimeoutSeconds": globalConfig.TCPServer.TCPWriteTimeoutSeconds,
				"writeTimeout":        retryConfig.WriteTimeout,
			}).Info("✅ TCP写入超时配置已从配置文件加载")
		}
	}

	return &DeviceGateway{
		tcpManager: core.GetGlobalTCPManager(),
		tcpWriter:  network.NewTCPWriter(retryConfig, logger.GetLogger()),
	}
}

// ===============================
// 设备连接管理接口
// ===============================

/**
 * @description: 判断设备是否在线
 * @param {string} deviceID
 * @return {bool}
 */
func (g *DeviceGateway) IsDeviceOnline(deviceID string) bool {
	if g.tcpManager == nil {
		return false
	}
	// 严格在线视图：存在即在线
	_, ok := g.tcpManager.GetDeviceByID(deviceID)
	return ok
}

/**
 * @description: 获取所有在线设备ID列表
 * @return {[]string}
 */
func (g *DeviceGateway) GetAllOnlineDevices() []string {
	logger.WithFields(logrus.Fields{"action": "GetAllOnlineDevices"}).Debug("start")

	var onlineDevices []string

	if g.tcpManager == nil {
		logger.WithFields(logrus.Fields{"action": "GetAllOnlineDevices", "error": "tcpManager nil"}).Debug("skip")
		return onlineDevices
	}

	groupCount := 0
	totalDevices := 0

	// 遍历所有设备组
	g.tcpManager.GetDeviceGroups().Range(func(key, value interface{}) bool {
		groupCount++
		_ = key.(string)
		deviceGroup := value.(*core.DeviceGroup)
		deviceGroup.RLock()

		// logger.WithFields(logrus.Fields{"action":"GetAllOnlineDevices","iccid":iccid,"deviceCount":len(deviceGroup.Devices)}).Trace("scan group")

		deviceInGroup := 0
		for deviceID, device := range deviceGroup.Devices {
			totalDevices++
			deviceInGroup++
			// logger.WithFields(logrus.Fields{"action":"GetAllOnlineDevices","deviceID":deviceID,"status":device.Status.String()}).Trace("scan device")

			if device.Status == constants.DeviceStatusOnline {
				onlineDevices = append(onlineDevices, deviceID)
			}
		}

		deviceGroup.RUnlock()
		return true
	})

	logger.WithFields(logrus.Fields{
		"action":       "GetAllOnlineDevices",
		"groupCount":   groupCount,
		"totalDevices": totalDevices,
		"onlineCount":  len(onlineDevices),
	}).Debug("获取所有在线设备列表")

	return onlineDevices
}

/**
 * @description: 统计在线设备数量
 * @return {int}
 */
func (g *DeviceGateway) CountOnlineDevices() int {
	return len(g.GetAllOnlineDevices())
}

/**
 * @description: 获取设备详细信息
 * @param {string} deviceID
 * @return {map[string]interface{}, error}
 */
func (g *DeviceGateway) GetDeviceDetail(deviceID string) (map[string]interface{}, error) {
	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
	}).Debug("开始获取设备详情")

	if g.tcpManager == nil {
		logger.WithFields(logrus.Fields{
			"action": "GetDeviceDetail",
			"error":  "TCP管理器未初始化",
		}).Error("获取设备详情失败")
		return nil, fmt.Errorf("TCP管理器未初始化")
	}

	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
	}).Debug("调用TCPManager.GetDeviceDetail")

	result, err := g.tcpManager.GetDeviceDetail(deviceID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"action":   "GetDeviceDetail",
			"deviceID": deviceID,
			"error":    err,
		}).Error("TCPManager返回错误")
		return nil, err
	}

	fmt.Printf("✅ [DeviceGateway.GetDeviceDetail] TCPManager返回成功: deviceID=%s, keys=%d\n", deviceID, len(result))
	return result, nil
}

/**
 * @description: 服务端主动断开设备连接
 * @param {string} deviceID
 * @return {bool}
 */
func (g *DeviceGateway) DisconnectDevice(deviceID string) bool {
	if g.tcpManager == nil {
		return false
	}
	ok := g.tcpManager.DisconnectByDeviceID(deviceID, "manual")
	if ok {
		logger.WithFields(logrus.Fields{"deviceID": deviceID}).Info("设备连接已主动断开并清理")
	}
	return ok
}

// ===============================
// 设备命令发送接口
// ===============================

/**
 * @description: 发送命令到指定设备
 * @param {string} deviceID
 * @param {byte} command
 * @param {[]byte} data
 * @return {error}
 */
func (g *DeviceGateway) SendCommandToDevice(deviceID string, command byte, data []byte) error {
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	conn, exists := g.tcpManager.GetConnectionByDeviceID(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不在线", deviceID)
	}

	// 🔧 修复：验证设备连接存在
	_, sessionExists := g.tcpManager.GetSessionByDeviceID(deviceID)
	if !sessionExists {
		return fmt.Errorf("设备会话不存在")
	}

	// 🔧 修复：验证设备ID与Session中的PhysicalID是否匹配
	expectedPhysicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 🔧 修复：从设备信息中获取PhysicalID，而不是从ConnectionSession
	device, deviceExists := g.tcpManager.GetDeviceByID(deviceID)
	if !deviceExists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	sessionPhysicalID := device.PhysicalID

	// 🔧 修复：验证一致性，如果不匹配则修复Device的PhysicalID
	if expectedPhysicalID != sessionPhysicalID {
		logger.WithFields(logrus.Fields{
			"deviceID":           deviceID,
			"expectedPhysicalID": utils.FormatPhysicalID(expectedPhysicalID),
			"devicePhysicalID":   utils.FormatPhysicalID(sessionPhysicalID),
			"action":             "FIXING_PHYSICAL_ID_MISMATCH",
		}).Warn("🔧 检测到PhysicalID不匹配，正在修复Device数据")

		// 🔧 修复：使用Device的mutex保护并发更新
		device.Lock()
		device.PhysicalID = expectedPhysicalID
		device.Unlock()

		// 同时修复设备组中的Device数据
		if err := g.fixDeviceGroupPhysicalID(deviceID, expectedPhysicalID); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    err,
			}).Error("修复设备组PhysicalID失败")
		}

		logger.WithFields(logrus.Fields{
			"deviceID":            deviceID,
			"correctedPhysicalID": utils.FormatPhysicalID(expectedPhysicalID),
		}).Info("✅ PhysicalID不匹配已修复")
	}

	// 使用API请求的正确PhysicalID，而不是Session中可能错误的值
	physicalID := expectedPhysicalID

	// 使用统一DNY构建器，确保使用小端序（符合AP3000协议规范）
	// 🔧 修复：使用动态MessageID避免重复，防止设备混乱
	messageID := pkg.Protocol.GetNextMessageID()
	builder := protocol.NewUnifiedDNYBuilder()
	dnyPacket := builder.BuildDNYPacket(physicalID, messageID, command, data)

	// 🔧 详细Hex数据日志 - 用于调试命令发送问题
	logger.WithFields(logrus.Fields{
		"deviceID":        deviceID,
		"physicalID":      utils.FormatPhysicalID(physicalID),
		"messageID":       fmt.Sprintf("0x%04X", messageID),
		"command":         fmt.Sprintf("0x%02X", command),
		"commandName":     g.getCommandName(command),
		"dataLen":         len(data),
		"dataHex":         fmt.Sprintf("%X", data),
		"packetHex":       fmt.Sprintf("%X", dnyPacket),
		"packetLen":       len(dnyPacket),
		"msgID":           messageID,
		"packetStructure": g.analyzePacketStructure(dnyPacket, physicalID, command, messageID),
		"byteOrder":       "小端序(Little-Endian)",
		"action":          "SEND_DNY_PACKET",
	}).Info("📡 发送DNY命令数据包 - 详细Hex记录")

	// �🚀 Phase 2: 使用TCPWriter发送数据包，支持重试机制
	if err := g.tcpWriter.WriteWithRetry(conn, 0, dnyPacket); err != nil {
		return fmt.Errorf("发送命令失败: %v", err)
	}

	// 记录命令元数据
	g.tcpManager.RecordDeviceCommand(deviceID, command, len(data))

	return nil
}

// fixDeviceGroupPhysicalID 修复设备组中Device的PhysicalID
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

/**
 * @description: 发送充电控制命令
 * @param {string} deviceID
 * @param {uint8} port 端口号(1-255)
 * @param {uint8} action 操作类型(0x01:开始充电, 0x00:停止充电)
 * @return {error}
 */
func (g *DeviceGateway) SendChargingCommand(deviceID string, port uint8, action uint8) error {
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
	}

	commandData := []byte{port, action}

	// 🔧 优化：统一日志字段格式，增加关键业务信息
	actionStr := "STOP_CHARGING"
	actionDesc := "停止充电"
	if action == 0x01 {
		actionStr = "START_CHARGING"
		actionDesc = "开始充电"
	}

	// 🔧 优化：发送前日志记录
	logFields := logrus.Fields{
		"deviceID":   deviceID,
		"command":    "CHARGE_CONTROL",
		"commandID":  fmt.Sprintf("0x%02X", constants.CmdChargeControl),
		"port":       port,
		"action":     actionStr,
		"actionCode": fmt.Sprintf("0x%02X", action),
		"actionDesc": actionDesc,
		"dataLen":    len(commandData),
		"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
	}

	logger.WithFields(logFields).Info("🔌 准备发送充电控制命令")

	err := g.SendCommandToDevice(deviceID, constants.CmdChargeControl, commandData)
	if err != nil {
		// 🔧 优化：失败日志增加详细信息
		logger.WithFields(logrus.Fields{
			"deviceID":   deviceID,
			"command":    "CHARGE_CONTROL",
			"commandID":  fmt.Sprintf("0x%02X", constants.CmdChargeControl),
			"port":       port,
			"action":     actionStr,
			"actionCode": fmt.Sprintf("0x%02X", action),
			"error":      err.Error(),
			"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
		}).Error("❌ 充电控制命令发送失败")
		return fmt.Errorf("发送充电控制命令失败: %v", err)
	}

	// 🔧 优化：成功日志增加业务上下文
	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"command":    "CHARGE_CONTROL",
		"commandID":  fmt.Sprintf("0x%02X", constants.CmdChargeControl),
		"port":       port,
		"action":     actionStr,
		"actionCode": fmt.Sprintf("0x%02X", action),
		"actionDesc": actionDesc,
		"status":     "SENT",
		"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
	}).Info("⚡ 充电控制命令发送成功")

	return nil
}

/**
 * @description: 发送完整参数的充电控制命令
 * @param {string} deviceID 设备ID
 * @param {uint8} port 端口号(1-255)
 * @param {uint8} action 操作类型(0x01:开始充电, 0x00:停止充电)
 * @param {string} orderNo 订单号
 * @param {uint8} mode 充电模式(0:按时间, 1:按电量)
 * @param {uint16} value 充电值(时间:分钟, 电量:0.1度)
 * @param {uint32} balance 余额(分)
 * @return {error}
 */
func (g *DeviceGateway) SendChargingCommandWithParams(deviceID string, port uint8, action uint8, orderNo string, mode uint8, value uint16, balance uint32) error {
	// 🔧 增强参数验证
	if deviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
	}

	// 订单号长度验证 - 协议限制16字节
	if len(orderNo) > 16 {
		return fmt.Errorf("订单号长度超过限制：当前%d字节，最大16字节，订单号：%s", len(orderNo), orderNo)
	}

	// 充电参数验证
	if mode == 0 && value == 0 {
		return fmt.Errorf("按时间充电时，充电时长不能为0秒")
	}
	if mode == 1 && value == 0 {
		return fmt.Errorf("按电量充电时，充电电量不能为0")
	}
	if mode > 1 {
		return fmt.Errorf("充电模式无效：%d，有效值：0(按时间)或1(按电量)", mode)
	}
	if action > 1 {
		return fmt.Errorf("充电动作无效：%d，有效值：0(停止)或1(开始)", action)
	}

	if balance == 0 {
		return fmt.Errorf("余额不能为0")
	}
	if value == 0 {
		return fmt.Errorf("充电值不能为0")
	}

	// 🔧 修复：使用正确的AP3000协议82指令格式（37字节）
	// 根据AP3000协议文档：费率模式 + 余额/有效期 + 端口号 + 充电命令 + 充电时长/电量 + 订单编号 + 其他参数
	commandData := make([]byte, 37)

	// 费率模式(1字节)：0=计时，1=包月，2=计量，3=计次
	commandData[0] = mode

	// 余额/有效期(4字节，小端序)
	commandData[1] = byte(balance)
	commandData[2] = byte(balance >> 8)
	commandData[3] = byte(balance >> 16)
	commandData[4] = byte(balance >> 24)

	// 端口号(1字节)：从0开始，0x00=第1路
	commandData[5] = port - 1 // API端口号是1-based，协议是0-based

	// 充电命令(1字节)：0=停止充电，1=开始充电
	commandData[6] = action

	// 🔧 修复：API传入的value已经是正确的单位（按时间=秒，按电量=0.1度）
	// 不需要进行单位转换，直接使用
	actualValue := value

	// 充电时长/电量(2字节，小端序)
	commandData[7] = byte(actualValue)
	commandData[8] = byte(actualValue >> 8)

	// 订单编号(16字节) - 🔧 修复：处理订单号长度超限问题
	orderBytes := make([]byte, 16)
	if len(orderNo) > 0 {
		copy(orderBytes, []byte(orderNo))
	}
	copy(commandData[9:25], orderBytes)

	// 🔧 修复：最大充电时长设置逻辑
	// 根据协议文档：如果参数为0表示不修改，会使用设备的设置值，默认10小时
	var maxChargeDuration uint16
	if mode == 0 && actualValue > 0 { // 按时间充电且有具体时长
		// 设置为充电时长的1.5倍，确保不会因为最大时长限制而提前停止
		maxChargeDuration = actualValue + (actualValue / 2)
		// 但不超过10小时（36000秒）
		if maxChargeDuration > 36000 {
			maxChargeDuration = 36000
		}
	} else {
		// 其他情况使用设备默认值
		maxChargeDuration = 0
	}
	commandData[25] = byte(maxChargeDuration)
	commandData[26] = byte(maxChargeDuration >> 8)

	// 过载功率(2字节，小端序)
	overloadPower := uint16(0) // 0表示不设置
	commandData[27] = byte(overloadPower)
	commandData[28] = byte(overloadPower >> 8)

	// 二维码灯(1字节)：0=打开，1=关闭
	commandData[29] = 0

	// 长充模式(1字节)：0=关闭，1=打开
	commandData[30] = 0

	// 额外浮充时间(2字节，小端序)：0=不开启
	commandData[31] = 0
	commandData[32] = 0

	// 是否跳过短路检测(1字节)：2=正常检测短路
	commandData[33] = 2

	// 不判断用户拔出(1字节)：0=正常判断拔出
	commandData[34] = 0

	// 强制带充满自停(1字节)：0=正常
	commandData[35] = 0

	// 充满功率(1字节)：0=关闭充满功率判断
	commandData[36] = 0

	err := g.SendCommandToDevice(deviceID, constants.CmdChargeControl, commandData)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %v", err)
	}

	actionStr := "停止充电"
	if action == 0x01 {
		actionStr = "开始充电"
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

	return nil
}

// getValueUnit 获取value字段的单位描述
func getValueUnit(mode uint8) string {
	if mode == 0 {
		return "秒"
	}
	return "0.1度"
}

/**
 * @description: 发送设备定位命令
 * @param {string} deviceID
 * @return {error}
 */
func (g *DeviceGateway) SendLocationCommand(deviceID string, locateTime int) error {
	// 🔧 修复：使用正确的设备定位命令(0x96)，添加定位时间参数
	// 定位时间：根据协议，1字节表示执行时长，单位秒
	locationDuration := byte(locateTime)

	// 🔧 优化：统一日志字段格式，增加关键业务信息
	logFields := logrus.Fields{
		"deviceID":       deviceID,
		"command":        "DEVICE_LOCATE",
		"commandID":      fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
		"locateTime":     locateTime,
		"actualDuration": locationDuration,
		"action":         "PREPARE_SEND",
		"timestamp":      time.Now().Format("2006-01-02 15:04:05"),
	}

	logger.WithFields(logFields).Info("🎯 准备发送设备定位命令")

	err := g.SendCommandToDevice(deviceID, constants.CmdDeviceLocate, []byte{locationDuration})
	if err != nil {
		// 🔧 优化：失败日志增加详细信息
		logger.WithFields(logrus.Fields{
			"deviceID":   deviceID,
			"command":    "DEVICE_LOCATE",
			"commandID":  fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
			"locateTime": locateTime,
			"error":      err.Error(),
			"action":     "SEND_FAILED",
			"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
		}).Error("❌ 设备定位命令发送失败")
		return fmt.Errorf("发送定位命令失败: %v", err)
	}

	// 🔧 优化：成功日志增加业务上下文
	logger.WithFields(logrus.Fields{
		"deviceID":         deviceID,
		"command":          "DEVICE_LOCATE",
		"commandID":        fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
		"locateTime":       locateTime,
		"duration":         locationDuration,
		"action":           "SEND_SUCCESS",
		"expectedBehavior": "设备将播放语音并闪灯",
		"timestamp":        time.Now().Format("2006-01-02 15:04:05"),
	}).Info("🔊 设备定位命令发送成功")
	return nil
}

/**
 * @description: 向所有在线设备广播消息
 * @param {byte} command
 * @param {[]byte} data
 * @return {int} 成功发送的设备数量
 */
func (g *DeviceGateway) BroadcastToAllDevices(command byte, data []byte) int {
	onlineDevices := g.GetAllOnlineDevices()
	successCount := 0

	for _, deviceID := range onlineDevices {
		if err := g.SendCommandToDevice(deviceID, command, data); err == nil {
			successCount++
		}
	}

	logger.WithFields(logrus.Fields{
		"command":      fmt.Sprintf("0x%02X", command),
		"totalDevices": len(onlineDevices),
		"successCount": successCount,
	}).Info("广播命令完成")

	return successCount
}

// ===============================
// 设备分组管理接口 (基于ICCID)
// ===============================

/**
 * @description: 获取指定ICCID下的所有设备
 * @param {string} iccid
 * @return {[]string}
 */
func (g *DeviceGateway) GetDevicesByICCID(iccid string) []string {
	var devices []string

	if g.tcpManager == nil {
		return devices
	}

	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return devices
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	for deviceID := range deviceGroup.Devices {
		devices = append(devices, deviceID)
	}

	return devices
}

/**
 * @description: 向指定ICCID组内所有设备发送命令
 * @param {string} iccid
 * @param {byte} command
 * @param {[]byte} data
 * @return {int, error} 成功发送数量, 错误信息
 */
func (g *DeviceGateway) SendCommandToGroup(iccid string, command byte, data []byte) (int, error) {
	devices := g.GetDevicesByICCID(iccid)
	if len(devices) == 0 {
		return 0, fmt.Errorf("ICCID %s 下没有设备", iccid)
	}

	successCount := 0
	for _, deviceID := range devices {
		if g.IsDeviceOnline(deviceID) {
			if err := g.SendCommandToDevice(deviceID, command, data); err == nil {
				successCount++
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"iccid":        iccid,
		"command":      fmt.Sprintf("0x%02X", command),
		"totalDevices": len(devices),
		"successCount": successCount,
	}).Info("组播命令完成")

	return successCount, nil
}

/**
 * @description: 统计指定ICCID组内的设备数量
 * @param {string} iccid
 * @return {int}
 */
func (g *DeviceGateway) CountDevicesInGroup(iccid string) int {
	return len(g.GetDevicesByICCID(iccid))
}

// ===============================
// 设备状态查询接口
// ===============================

/**
 * @description: 获取设备状态
 * @param {string} deviceID
 * @return {string, bool} 状态字符串, 是否存在
 */
func (g *DeviceGateway) GetDeviceStatus(deviceID string) (string, bool) {
	if g.tcpManager == nil {
		return "", false
	}

	iccidInterface, exists := g.tcpManager.GetDeviceIndex().Load(deviceID)
	if !exists {
		return "", false
	}

	iccid := iccidInterface.(string)
	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return "", false
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	device, exists := deviceGroup.Devices[deviceID]
	if !exists {
		return "", false
	}

	return device.Status.String(), true
}

/**
 * @description: 获取设备最后心跳时间
 * @param {string} deviceID
 * @return {time.Time}
 */
func (g *DeviceGateway) GetDeviceHeartbeat(deviceID string) time.Time {
	if g.tcpManager == nil {
		return time.Time{}
	}

	iccidInterface, exists := g.tcpManager.GetDeviceIndex().Load(deviceID)
	if !exists {
		return time.Time{}
	}

	iccid := iccidInterface.(string)
	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return time.Time{}
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	device, exists := deviceGroup.Devices[deviceID]
	if !exists {
		return time.Time{}
	}

	return device.LastHeartbeat
}

/**
 * @description: 获取网关统计信息
 * @return {map[string]interface{}}
 */
func (g *DeviceGateway) GetDeviceStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	if g.tcpManager == nil {
		stats["error"] = "TCP管理器未初始化"
		return stats
	}

	// 基础统计
	onlineDevices := g.GetAllOnlineDevices()
	stats["onlineDeviceCount"] = len(onlineDevices)
	stats["onlineDevices"] = onlineDevices

	// 连接统计
	connectionCount := int64(0)
	g.tcpManager.GetConnections().Range(func(key, value interface{}) bool {
		connectionCount++
		return true
	})
	stats["connectionCount"] = connectionCount

	// 设备组统计
	groupCount := int64(0)
	totalDevices := int64(0)
	g.tcpManager.GetDeviceGroups().Range(func(key, value interface{}) bool {
		groupCount++
		deviceGroup := value.(*core.DeviceGroup)
		deviceGroup.RLock()
		totalDevices += int64(len(deviceGroup.Devices))
		deviceGroup.RUnlock()
		return true
	})
	stats["groupCount"] = groupCount
	stats["totalDeviceCount"] = totalDevices

	// 时间统计
	stats["timestamp"] = time.Now().Unix()
	stats["formattedTime"] = time.Now().Format("2006-01-02 15:04:05")

	return stats
}

// ===============================
// 全局网关实例管理
// ===============================

var globalDeviceGateway *DeviceGateway

// GetGlobalDeviceGateway 获取全局设备网关实例
func GetGlobalDeviceGateway() *DeviceGateway {
	if globalDeviceGateway == nil {
		globalDeviceGateway = NewDeviceGateway()
		logger.Info("全局设备网关已初始化")
	}
	return globalDeviceGateway
}

// InitializeGlobalDeviceGateway 初始化全局设备网关
func InitializeGlobalDeviceGateway() {
	globalDeviceGateway = NewDeviceGateway()
	logger.Info("全局设备网关初始化完成")
}

// ===============================
// 调试和日志辅助方法
// ===============================

// getCommandName 获取命令名称（用于日志记录）
func (g *DeviceGateway) getCommandName(command byte) string {
	switch command {
	case 0x96:
		return "CmdDeviceLocate(声光寻找设备)"
	case 0x82:
		return "CmdChargeControl(充电控制)"
	case 0x81:
		return "CmdQueryDeviceStatus(查询设备状态)"
	default:
		return fmt.Sprintf("Unknown(0x%02X)", command)
	}
}

// analyzePacketStructure 分析数据包结构（用于调试）
func (g *DeviceGateway) analyzePacketStructure(packet []byte, physicalID uint32, command byte, messageID uint16) string {
	if len(packet) < 12 {
		return "数据包长度不足"
	}

	return fmt.Sprintf("Header=DNY, Length=%d, PhysicalID=0x%08X, MessageID=, Command=0x%02X",
		len(packet)-5, physicalID, messageID, command)
}
