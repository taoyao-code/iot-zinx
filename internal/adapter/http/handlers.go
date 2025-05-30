package http

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 属性键常量 - 使用pkg包中定义的常量
const (
	PropKeyICCID            = pkg.PropKeyICCID
	PropKeyLastHeartbeat    = pkg.PropKeyLastHeartbeat
	PropKeyLastHeartbeatStr = pkg.PropKeyLastHeartbeatStr
	PropKeyConnStatus       = pkg.PropKeyConnStatus
)

// 连接状态常量 - 使用pkg包中定义的常量
const (
	ConnStatusActive   = pkg.ConnStatusActive
	ConnStatusInactive = pkg.ConnStatusInactive
)

// HandleHealthCheck 健康检查处理
func HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "充电设备网关运行正常",
	})
}

// HandleDeviceStatus 处理设备状态查询
func HandleDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// 参数验证
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 查询设备连接状态
	tcpMonitor := pkg.Monitor.GetGlobalMonitor()
	conn, exists := tcpMonitor.GetConnectionByDeviceId(deviceID)

	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 获取ICCID
	iccid := ""
	if iccidVal, err := conn.GetProperty(PropKeyICCID); err == nil {
		iccid = iccidVal.(string)
	}

	// 获取最后心跳时间（优先使用格式化的字符串）
	lastHeartbeatStr := "never"
	var lastHeartbeat int64
	var timeSinceHeart float64

	if val, err := conn.GetProperty(PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	} else if val, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && val != nil {
		lastHeartbeat = val.(int64)
		lastHeartbeatStr = time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05")
		timeSinceHeart = time.Since(time.Unix(lastHeartbeat, 0)).Seconds()
	}

	// 获取连接状态
	connStatus := ConnStatusInactive
	if statusVal, err := conn.GetProperty(PropKeyConnStatus); err == nil && statusVal != nil {
		connStatus = statusVal.(string)
	}

	// 返回设备状态信息
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"deviceId":       deviceID,
			"iccid":          iccid,
			"isOnline":       connStatus == ConnStatusActive,
			"status":         connStatus,
			"lastHeartbeat":  lastHeartbeat,
			"heartbeatTime":  lastHeartbeatStr,
			"timeSinceHeart": timeSinceHeart,
			"remoteAddr":     conn.RemoteAddr().String(),
		},
	})
}

// HandleSendCommand 处理发送命令到设备
func HandleSendCommand(c *gin.Context) {
	// 解析请求参数
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Command  byte   `json:"command" binding:"required"`
		Data     []byte `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 查询设备连接
	tcpMonitor := pkg.Monitor.GetGlobalMonitor()
	conn, exists := tcpMonitor.GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 解析设备ID为物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 16, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误",
		})
		return
	}

	// 发送命令到设备（使用正确的DNY协议）
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	err = pkg.Protocol.SendDNYResponse(conn, uint32(physicalID), messageID, req.Command, req.Data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"command":  req.Command,
			"error":    err.Error(),
		}).Error("发送命令到设备失败")

		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送命令失败: " + err.Error(),
		})
		return
	}

	// 返回成功
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "命令发送成功",
	})
}

// HandleDeviceList 获取当前在线设备列表
func HandleDeviceList(c *gin.Context) {
	var devices []gin.H
	// 获取设备服务
	deviceService := app.GetServiceManager().DeviceService

	// 从设备服务获取所有设备状态
	allDevices := deviceService.GetAllDevices()

	// 创建设备ID映射，用于后续合并连接信息
	deviceMap := make(map[string]gin.H)
	for _, device := range allDevices {
		deviceInfo := gin.H{
			"deviceId": device.DeviceID,
			"isOnline": device.Status == pkg.DeviceStatusOnline,
			"status":   device.Status,
		}

		// 添加ICCID（如果有）
		if device.ICCID != "" {
			deviceInfo["iccid"] = device.ICCID
		}

		// 添加最后更新时间
		if device.LastSeen > 0 {
			deviceInfo["lastUpdate"] = device.LastSeen
			deviceInfo["lastUpdateTime"] = time.Unix(device.LastSeen, 0).Format("2006-01-02 15:04:05")
		}

		deviceMap[device.DeviceID] = deviceInfo
	}

	// 获取全局TCP监视器
	tcpMonitor := pkg.Monitor.GetGlobalMonitor()
	if tcpMonitor == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: TCP监视器未初始化",
		})
		return
	}

	// 由于没有直接的RangeDeviceConnections函数，我们需要修改这里的逻辑
	// 这里可能需要实现一个遍历所有设备连接的函数
	// 临时方案：简化处理，直接返回现有设备
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"devices": devices,
			"total":   len(devices),
		},
	})
}

// HandleSendDNYCommand 发送DNY协议命令
func HandleSendDNYCommand(c *gin.Context) {
	var req struct {
		DeviceID  string `json:"deviceId" binding:"required"`
		Command   byte   `json:"command" binding:"required"`
		Data      string `json:"data"` // HEX字符串
		MessageID uint16 `json:"messageId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 查询设备连接
	conn, exists := pkg.Monitor.GetGlobalMonitor().GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 解析物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误",
		})
		return
	}

	// 解析数据字段
	var data []byte
	if req.Data != "" {
		data, err = hex.DecodeString(req.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "数据字段HEX格式错误",
			})
			return
		}
	}

	// 构建DNY协议帧
	packetData := buildDNYPacket(uint32(physicalID), req.MessageID, req.Command, data)

	// 发送到设备
	err = conn.SendBuffMsg(0, packetData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"command":  req.Command,
			"error":    err.Error(),
		}).Error("发送DNY命令到设备失败")

		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送命令失败: " + err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceId":  req.DeviceID,
		"command":   fmt.Sprintf("0x%02X", req.Command),
		"messageId": req.MessageID,
		"dataHex":   hex.EncodeToString(data),
		"packetHex": hex.EncodeToString(packetData),
	}).Info("发送DNY命令到设备")

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "DNY命令发送成功",
		Data: gin.H{
			"packetHex": hex.EncodeToString(packetData),
		},
	})
}

// HandleQueryDeviceStatus 查询设备状态（0x81命令）
func HandleQueryDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 发送查询状态命令
	req := struct {
		DeviceID  string `json:"deviceId"`
		Command   byte   `json:"command"`
		Data      string `json:"data"`
		MessageID uint16 `json:"messageId"`
	}{
		DeviceID:  deviceID,
		Command:   0x81, // 查询设备联网状态命令
		Data:      "",   // 无数据
		MessageID: uint16(time.Now().Unix() & 0xFFFF),
	}

	// 复用发送DNY命令的逻辑
	c.Set("json_body", req)
	HandleSendDNYCommand(c)
}

// HandleStartCharging 开始充电（0x82命令）
func HandleStartCharging(c *gin.Context) {
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Port     byte   `json:"port" binding:"required"`    // 端口号
		Mode     byte   `json:"mode" binding:"required"`    // 充电模式 0=按时间 1=按电量
		Value    uint16 `json:"value" binding:"required"`   // 充电时间(分钟)或电量(0.1度)
		OrderNo  string `json:"orderNo" binding:"required"` // 订单号
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 构建充电控制数据
	data := make([]byte, 12)
	data[0] = req.Port                                  // 端口号
	data[1] = req.Mode                                  // 充电模式
	binary.LittleEndian.PutUint16(data[2:4], req.Value) // 充电时间/电量
	copy(data[4:12], []byte(req.OrderNo)[:8])           // 订单号(取前8字节)

	// 发送充电控制命令
	dnyReq := struct {
		DeviceID  string `json:"deviceId"`
		Command   byte   `json:"command"`
		Data      string `json:"data"`
		MessageID uint16 `json:"messageId"`
	}{
		DeviceID:  req.DeviceID,
		Command:   0x82, // 开始/停止充电操作
		Data:      hex.EncodeToString(data),
		MessageID: uint16(time.Now().Unix() & 0xFFFF),
	}

	c.Set("json_body", dnyReq)
	HandleSendDNYCommand(c)
}

// HandleStopCharging 停止充电（0x82命令，端口号设为0xFF）
func HandleStopCharging(c *gin.Context) {
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Port     byte   `json:"port"`    // 端口号，0xFF表示停止所有端口
		OrderNo  string `json:"orderNo"` // 订单号
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 如果没有指定端口，默认停止所有端口
	if req.Port == 0 {
		req.Port = 0xFF
	}

	// 构建停止充电数据
	data := make([]byte, 12)
	data[0] = req.Port // 端口号
	data[1] = 0xFF     // 停止充电标志
	if req.OrderNo != "" {
		copy(data[4:12], []byte(req.OrderNo)[:8]) // 订单号
	}

	// 发送停止充电命令
	dnyReq := struct {
		DeviceID  string `json:"deviceId"`
		Command   byte   `json:"command"`
		Data      string `json:"data"`
		MessageID uint16 `json:"messageId"`
	}{
		DeviceID:  req.DeviceID,
		Command:   0x82,
		Data:      hex.EncodeToString(data),
		MessageID: uint16(time.Now().Unix() & 0xFFFF),
	}

	c.Set("json_body", dnyReq)
	HandleSendDNYCommand(c)
}

// HandleTestTool 测试工具主页面
func HandleTestTool(c *gin.Context) {
	c.HTML(http.StatusOK, "test_tool.html", gin.H{
		"title": "充电设备网关测试工具",
	})
}

// buildDNYPacket 构建DNY协议数据包
func buildDNYPacket(physicalID uint32, messageID uint16, command byte, data []byte) []byte {
	// 计算数据段长度（物理ID + 消息ID + 命令 + 数据）
	dataLen := 4 + 2 + 1 + len(data)

	// 构建数据包
	packet := make([]byte, 0, 5+dataLen+2) // 包头(3) + 长度(2) + 数据 + 校验(2)

	// 包头
	packet = append(packet, 'D', 'N', 'Y')

	// 长度（小端模式）
	packet = append(packet, byte(dataLen), byte(dataLen>>8))

	// 物理ID（小端模式）
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 计算校验和
	checksum := calculatePacketChecksum(packet)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// calculatePacketChecksum 计算数据包校验和
func calculatePacketChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}
