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
	"sync"
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
	// AP3000 节流：同设备命令间隔≥0.5秒
	throttleMu       sync.Mutex
	lastSendByDevice map[string]time.Time

	// 订单上下文缓存：deviceID|protocolPort(0-based) → ctx
	orderCtxMu sync.RWMutex
	orderCtx   map[string]OrderContext
}

// OrderContext 当前订单上下文（用于仅更新0x82时回填）
type OrderContext struct {
	OrderNo string
	Mode    uint8  // 0=计时,1=包月,2=计量,3=计次
	Value   uint16 // 时长(秒)或电量(0.1度)
	Balance uint32 // 余额/有效期(4B)
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
		tcpManager:       core.GetGlobalTCPManager(),
		tcpWriter:        network.NewTCPWriter(retryConfig, logger.GetLogger()),
		lastSendByDevice: make(map[string]time.Time),
		orderCtx:         make(map[string]OrderContext),
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

	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
		"keys":     len(result),
	}).Debug("TCPManager返回成功")
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

	// 🔧 修复：验证设备连接存在
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

	// 充电参数验证（开始充电严格、停止充电放宽）
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

	// 🔧 修复：最大充电时长设置逻辑（停止命令不修改）
	// 根据协议文档：如果参数为0表示不修改，会使用设备的设置值，默认10小时
	var maxChargeDuration uint16
	if action == 0x01 {
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
	} else {
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

	// 写入订单上下文（开始充电且订单号存在）
	if action == 0x01 && orderNo != "" {
		key := g.makeOrderCtxKey(deviceID, int(port-1))
		g.orderCtxMu.Lock()
		g.orderCtx[key] = OrderContext{OrderNo: orderNo, Mode: mode, Value: actualValue, Balance: balance}
		g.orderCtxMu.Unlock()
	}

	return nil
}

func (g *DeviceGateway) makeOrderCtxKey(deviceID string, protocolPort int) string {
	return fmt.Sprintf("%s|%d", deviceID, protocolPort)
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

	return fmt.Sprintf("Header=DNY, Length=%d, PhysicalID=0x%08X, MessageID=0x%04X, Command=0x%02X",
		len(packet)-5, physicalID, messageID, command)
}

// SendStopChargingCommand 发送停止充电命令（0x82，action=0x00）
// 最小负载：仅携带必要字段（端口、订单号），其余由设备忽略
func (g *DeviceGateway) SendStopChargingCommand(deviceID string, port uint8, orderNo string) error {
	return g.SendChargingCommandWithParams(deviceID, port, 0x00, orderNo, 0, 0, 0)
}

// UpdateChargingOverloadPower 在不改变当前订单其它参数的前提下，仅更新过载功率/最大充电时长
// 注意：
// - 需保持充电命令=1、订单号不变、端口号为协议0基(对外1基需减1)
// - 若不想调整最大充电时长，请传 maxChargeDurationSeconds=0 表示不修改
// - overloadPowerW 单位为瓦，对应0x82中的过载功率(2字节小端，单位为瓦)
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

	// 回填订单上下文
	mode := uint8(0)
	value := uint16(0)
	balance := uint32(0)
	if orderNo != "" {
		key := g.makeOrderCtxKey(deviceID, int(port-1))
		g.orderCtxMu.RLock()
		ctx, ok := g.orderCtx[key]
		g.orderCtxMu.RUnlock()
		if ok && ctx.OrderNo == orderNo {
			mode = ctx.Mode
			value = ctx.Value
			balance = ctx.Balance
		}
	}

	payload := make([]byte, 37)
	payload[0] = mode // 费率模式
	// 余额/有效期(4B)
	payload[1] = byte(balance)
	payload[2] = byte(balance >> 8)
	payload[3] = byte(balance >> 16)
	payload[4] = byte(balance >> 24)
	// 端口(协议0基)
	payload[5] = port - 1
	// 充电命令=1(保持充电)
	payload[6] = 0x01
	// 充电时长/电量(2B)
	payload[7] = byte(value)
	payload[8] = byte(value >> 8)
	// 订单号
	orderBytes := make([]byte, 16)
	copy(orderBytes, []byte(orderNo))
	copy(payload[9:25], orderBytes)
	// 最大充电时长(2B)：0表示不修改
	payload[25] = byte(maxChargeDurationSeconds)
	payload[26] = byte(maxChargeDurationSeconds >> 8)
	// 过载功率(2B) 单位瓦
	payload[27] = byte(overloadPowerW)
	payload[28] = byte(overloadPowerW >> 8)
	// 其余按默认
	payload[29] = 0                 // 二维码灯
	payload[30] = 0                 // 长充模式
	payload[31], payload[32] = 0, 0 // 额外浮充时间
	payload[33] = 2                 // 是否跳过短路检测=2正常
	payload[34] = 0                 // 不判断用户拔出
	payload[35] = 0                 // 强制带充满自停
	payload[36] = 0                 // 充满功率(单位1W)，此处关闭

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
