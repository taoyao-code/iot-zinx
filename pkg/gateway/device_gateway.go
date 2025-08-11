/*
 * @Author: IoT-Zinx团队
 * @Date: 2025-08-08 16:00:00
 * @LastEditors: IoT-Zinx团队
 * @LastEditTime: 2025-08-08 16:00:00
 * @Description: 设备网关统一接口层
 *
 * 【重要！！！重要！！！重要！！！】
 * 这里是IoT设备网关的核心组件库！
 * 借鉴WebSocket网关的简洁设计理念，提供统一的设备管理接口
 * 请谨慎修改此处的代码，除非你知道这意味着什么！
 */

package gateway

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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
	return &DeviceGateway{
		tcpManager: core.GetGlobalTCPManager(),
		tcpWriter:  network.NewTCPWriter(network.DefaultRetryConfig, logger.GetLogger()), // 🚀 Phase 2: 初始化TCPWriter
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
	fmt.Printf("🔍 [DeviceGateway.GetAllOnlineDevices] 开始获取在线设备列表\n")

	var onlineDevices []string

	if g.tcpManager == nil {
		fmt.Printf("❌ [DeviceGateway.GetAllOnlineDevices] TCP管理器未初始化\n")
		return onlineDevices
	}

	groupCount := 0
	totalDevices := 0

	// 遍历所有设备组
	g.tcpManager.GetDeviceGroups().Range(func(key, value interface{}) bool {
		groupCount++
		iccid := key.(string)
		deviceGroup := value.(*core.DeviceGroup)
		deviceGroup.RLock()

		fmt.Printf("🔍 [DeviceGateway.GetAllOnlineDevices] 检查设备组 %d: iccid=%s, 设备数=%d\n",
			groupCount, iccid, len(deviceGroup.Devices))

		deviceInGroup := 0
		for deviceID, device := range deviceGroup.Devices {
			totalDevices++
			deviceInGroup++
			fmt.Printf("🔍 [DeviceGateway.GetAllOnlineDevices] 设备 %d: deviceID=%s, status=%s, isOnline=%t\n",
				deviceInGroup, deviceID, device.Status.String(), device.Status == constants.DeviceStatusOnline)

			if device.Status == constants.DeviceStatusOnline {
				onlineDevices = append(onlineDevices, deviceID)
				fmt.Printf("✅ [DeviceGateway.GetAllOnlineDevices] 添加在线设备: deviceID=%s\n", deviceID)
			}
		}

		deviceGroup.RUnlock()
		return true
	})

	fmt.Printf("✅ [DeviceGateway.GetAllOnlineDevices] 扫描完成: 设备组数=%d, 总设备数=%d, 在线设备数=%d\n",
		groupCount, totalDevices, len(onlineDevices))
	fmt.Printf("✅ [DeviceGateway.GetAllOnlineDevices] 在线设备列表: %v\n", onlineDevices)

	logger.WithFields(logrus.Fields{
		"onlineCount": len(onlineDevices),
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
	fmt.Printf("🔍 [DeviceGateway.GetDeviceDetail] 开始获取设备详情: deviceID=%s\n", deviceID)

	if g.tcpManager == nil {
		fmt.Printf("❌ [DeviceGateway.GetDeviceDetail] TCP管理器未初始化\n")
		return nil, fmt.Errorf("TCP管理器未初始化")
	}

	fmt.Printf("🔍 [DeviceGateway.GetDeviceDetail] 调用TCPManager.GetDeviceDetail: deviceID=%s\n", deviceID)
	result, err := g.tcpManager.GetDeviceDetail(deviceID)
	if err != nil {
		fmt.Printf("❌ [DeviceGateway.GetDeviceDetail] TCPManager返回错误: deviceID=%s, error=%v\n", deviceID, err)
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

	// 构建DNY协议数据包
	session, sessionExists := g.tcpManager.GetSessionByDeviceID(deviceID)
	if !sessionExists {
		return fmt.Errorf("设备会话不存在")
	}

	// 统一：直接使用uint32格式的PhysicalID
	physicalID := session.PhysicalID

	// 使用统一DNY构建器
	builder := protocol.NewUnifiedDNYBuilder()
	dnyPacket := builder.BuildDNYPacket(physicalID, 0x0001, command, data)

	// � 详细Hex数据日志 - 用于调试命令发送问题
	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": utils.FormatPhysicalIDForLog(physicalID),
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%X", data),
		"packetHex":  fmt.Sprintf("%X", dnyPacket),
		"packetLen":  len(dnyPacket),
	}).Info("📡 发送DNY命令数据包 - 详细Hex记录")

	// �🚀 Phase 2: 使用TCPWriter发送数据包，支持重试机制
	if err := g.tcpWriter.WriteWithRetry(conn, 0, dnyPacket); err != nil {
		return fmt.Errorf("发送命令失败: %v", err)
	}

	// 记录命令元数据
	g.tcpManager.RecordDeviceCommand(deviceID, command, len(data))

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
	if port == 0 {
		return fmt.Errorf("端口号不能为0")
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

	// 充电时长/电量(2字节，小端序)
	commandData[7] = byte(value)
	commandData[8] = byte(value >> 8)

	// 订单编号(16字节)
	orderBytes := make([]byte, 16)
	if len(orderNo) > 0 {
		copy(orderBytes, []byte(orderNo))
	}
	copy(commandData[9:25], orderBytes)

	// 最大充电时长(2字节，小端序)
	maxChargeDuration := uint16(0) // 0表示不限制
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
		"deviceID": deviceID,
		"port":     port,
		"action":   actionStr,
		"orderNo":  orderNo,
		"mode":     modeStr,
		"value":    value,
		"balance":  balance,
	}).Info("完整参数充电控制命令发送成功")

	return nil
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
 * @description: 发送通用设备命令
 * @param {string} deviceID 设备ID
 * @param {string} command 命令类型
 * @param {map[string]interface{}} data 命令数据
 * @return {error}
 */
func (g *DeviceGateway) SendGenericCommand(deviceID, command string, data map[string]interface{}) error {
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	// 检查设备是否在线
	if !g.IsDeviceOnline(deviceID) {
		return fmt.Errorf("设备 %s 不在线", deviceID)
	}

	// 获取设备连接
	conn, exists := g.tcpManager.GetDeviceConnection(deviceID)
	if !exists {
		return fmt.Errorf("无法获取设备 %s 的连接", deviceID)
	}

	// 记录日志
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"command":  command,
		"data":     data,
	}).Info("发送通用设备命令")

	// 这里应该根据具体的协议来构造命令包
	// 暂时使用简单的方式，实际项目中需要根据协议规范实现
	commandData := map[string]interface{}{
		"command": command,
		"data":    data,
	}

	// 🚀 Phase 2: 使用TCPWriter发送命令，支持重试机制
	if err := g.tcpWriter.WriteWithRetry(conn, 0x01, []byte(fmt.Sprintf("%v", commandData))); err != nil {
		return fmt.Errorf("发送命令失败: %v", err)
	}
	// 记录命令
	g.tcpManager.RecordDeviceCommand(deviceID, 0x01, len(commandData))

	return nil
}

/**
 * @description: 发送DNY协议命令
 * @param {string} deviceID 设备ID
 * @param {string} command 命令类型
 * @param {string} data 命令数据
 * @return {error}
 */
func (g *DeviceGateway) SendDNYCommand(deviceID, command, data string) error {
	if g.tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	// 检查设备是否在线
	if !g.IsDeviceOnline(deviceID) {
		return fmt.Errorf("设备 %s 不在线", deviceID)
	}

	// 获取设备连接
	conn, exists := g.tcpManager.GetDeviceConnection(deviceID)
	if !exists {
		return fmt.Errorf("无法获取设备 %s 的连接", deviceID)
	}

	// 记录日志
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"command":  command,
		"data":     data,
		"data_hex": hex.EncodeToString([]byte(data)),
	}).Info("发送DNY协议命令")

	// 这里应该使用DNY协议构造器来构造命令包
	// 暂时使用简单的方式，实际项目中需要使用protocol包中的DNY构造器
	dnyCommand := fmt.Sprintf("DNY:%s:%s", command, data)

	// 🚀 Phase 2: 使用TCPWriter发送DNY命令，支持重试机制
	if err := g.tcpWriter.WriteWithRetry(conn, 0x02, []byte(dnyCommand)); err != nil {
		return fmt.Errorf("发送DNY命令失败: %v", err)
	}
	// 记录命令
	g.tcpManager.RecordDeviceCommand(deviceID, 0x02, len(dnyCommand))

	return nil
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
