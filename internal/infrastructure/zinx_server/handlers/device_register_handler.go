package handlers

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册请求 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	DNYHandlerBase
}

// 预处理
func (h *DeviceRegisterHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备注册请求")
}

// Handle 处理设备注册请求
func (h *DeviceRegisterHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理设备注册请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	// dnyMessageId := dnyMsg.GetDnyMessageId() // 暂不使用

	// 解析设备注册数据
	data := dnyMsg.GetData()
	registerData := &dny_protocol.DeviceRegisterData{}
	if err := registerData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("设备注册数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalId":      fmt.Sprintf("0x%08X", physicalId),
		"iccid":           registerData.ICCID,
		"deviceType":      registerData.DeviceType,
		"deviceVersion":   string(registerData.DeviceVersion[:]),
		"heartbeatPeriod": registerData.HeartbeatPeriod,
	}).Info("收到设备注册请求")

	// 将设备ID绑定到连接
	deviceIdStr := fmt.Sprintf("%08X", physicalId)

	// 存储ICCID
	iccid := registerData.ICCID
	conn.SetProperty(PropKeyICCID, iccid)

	// 检查是否存在会话
	sessionManager := monitor.GetSessionManager()
	var session *monitor.DeviceSession
	var isReconnect bool

	// 先尝试使用ICCID查找会话
	if iccid != "" && len(iccid) > 0 {
		if existSession, exists := sessionManager.GetSessionByICCID(iccid); exists {
			oldDeviceID := existSession.DeviceID

			// 设备ID变更，记录日志
			if oldDeviceID != deviceIdStr {
				logger.WithFields(logrus.Fields{
					"oldDeviceID": oldDeviceID,
					"newDeviceID": deviceIdStr,
					"iccid":       iccid,
					"sessionID":   existSession.SessionID,
				}).Info("设备ID已变更，但ICCID相同，可能是设备重启或更换了物理ID")

				// 添加临时ID映射，便于后续查找
				sessionManager.AddTempDeviceID(oldDeviceID, deviceIdStr)
			}

			session = existSession
			isReconnect = true
		}
	}

	// 再尝试使用设备ID查找会话
	if session == nil {
		if existSession, exists := sessionManager.GetSession(deviceIdStr); exists {
			session = existSession
			isReconnect = true
		}
	}

	// 绑定设备ID到连接
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceIdStr, conn)

	// 通知业务层设备上线
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceOnline(deviceIdStr, iccid)

	// 构建响应数据
	responseData := make([]byte, 5)
	responseData[0] = dny_protocol.ResponseSuccess        // 成功
	responseData[1] = uint8(registerData.DeviceType)      // 设备类型
	responseData[2] = uint8(registerData.DeviceType >> 8) // 设备类型高位
	responseData[3] = 0                                   // 预留
	responseData[4] = 0                                   // 预留

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdDeviceRegister), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送设备注册响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"deviceId":    deviceIdStr,
		"isReconnect": isReconnect,
	}).Debug("设备注册响应发送成功")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理设备注册请求
func (h *DeviceRegisterHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("设备注册请求处理完成")
}
