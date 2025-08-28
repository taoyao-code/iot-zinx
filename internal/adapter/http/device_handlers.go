package http

import (
	"net/http"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/gin-gonic/gin"
)

// DeviceHandlers 设备相关 HTTP 处理器
type DeviceHandlers struct {
	deviceGateway *gateway.DeviceGateway
}

func NewDeviceHandlers() *DeviceHandlers {
	return &DeviceHandlers{deviceGateway: gateway.GetGlobalDeviceGateway()}
}

// HandleDeviceStatus 获取设备状态
func (h *DeviceHandlers) HandleDeviceStatus(c *gin.Context) {
	var uri DeviceStatusURI
	if err := c.ShouldBindUri(&uri); err != nil || uri.DeviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "设备ID不能为空"})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(uri.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)"}})
		return
	}
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, APIResponse{Code: 404, Message: "设备不在线", Data: gin.H{"deviceId": uri.DeviceID, "standardId": standardDeviceID, "isOnline": false}})
		return
	}
	detail, err := h.deviceGateway.GetDeviceDetail(standardDeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: 500, Message: "获取设备信息失败"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "成功", Data: detail})
}

// HandleDeviceList 获取设备列表
func (h *DeviceHandlers) HandleDeviceList(c *gin.Context) {
	var q DeviceListQuery
	_ = c.ShouldBindQuery(&q)
	onlineDevices := h.deviceGateway.GetAllOnlineDevices()
	var deviceList []map[string]interface{}
	for _, deviceID := range onlineDevices {
		if detail, err := h.deviceGateway.GetDeviceDetail(deviceID); err == nil {
			deviceList = append(deviceList, detail)
		}
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "成功", Data: gin.H{"devices": deviceList, "total": len(onlineDevices), "online": len(onlineDevices)}})
}

// HandleQueryDeviceStatus 查询设备状态（与 HandleDeviceStatus 类似）
func (h *DeviceHandlers) HandleQueryDeviceStatus(c *gin.Context) {
	var uri DeviceStatusURI
	if err := c.ShouldBindUri(&uri); err != nil || uri.DeviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "设备ID不能为空"})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(uri.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)"}})
		return
	}
	detail, err := h.deviceGateway.GetDeviceDetail(standardDeviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: 404, Message: "设备不存在或离线"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "获取设备状态成功", Data: detail})
}

// HandleDeviceLocate 设备定位
func (h *DeviceHandlers) HandleDeviceLocate(c *gin.Context) {
	var req DeviceLocateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)"}})
		return
	}
	if err := h.deviceGateway.SendLocationCommand(standardDeviceID, int(req.LocateTime)); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: 500, Message: "发送定位命令失败: " + err.Error()})
		return
	}
	resp := ChargingActionResponse{ // 复用统一动作响应壳，字段兼容
		DeviceID:   req.DeviceID,
		StandardID: standardDeviceID,
		Port:       0,
		OrderNo:    "",
		Action:     "locate",
		Timestamp:  0,
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "定位命令发送成功", Data: gin.H{"deviceId": resp.DeviceID, "standardId": resp.StandardID, "action": resp.Action, "locateTime": req.LocateTime}})
}
