package http

import (
	"net/http"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/redis"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

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
	conn, exists := zinx_server.GetConnectionByDeviceId(deviceID)

	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 获取ICCID
	iccid := ""
	if iccidVal, err := conn.GetProperty(zinx_server.PropKeyICCID); err == nil {
		iccid = iccidVal.(string)
	}

	// 获取最后心跳时间
	lastHeartbeat := int64(0)
	if timeVal, err := conn.GetProperty(zinx_server.PropKeyLastHeartbeat); err == nil {
		lastHeartbeat = timeVal.(int64)
	}

	// 返回设备状态信息
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"deviceId":      deviceID,
			"iccid":         iccid,
			"isOnline":      true,
			"lastHeartbeat": lastHeartbeat,
			"remoteAddr":    conn.RemoteAddr().String(),
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
	conn, exists := zinx_server.GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 发送命令到设备
	err := conn.SendMsg(uint32(req.Command), req.Data)
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
	// 使用Redis获取在线设备信息
	redisClient := redis.GetClient()
	if redisClient == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "Redis连接不可用",
		})
		return
	}

	// TODO: 实现从Redis获取设备列表的逻辑
	// 目前返回一个空列表
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"devices": []interface{}{},
			"total":   0,
		},
	})
}
