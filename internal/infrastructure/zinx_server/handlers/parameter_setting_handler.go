package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ParameterSettingHandler 处理参数设置 (命令ID: 0x83, 0x84)
type ParameterSettingHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理参数设置
func (h *ParameterSettingHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到参数设置")
}

// Handle 处理参数设置
func (h *ParameterSettingHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理参数设置")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析参数设置数据
	data := dnyMsg.GetData()
	paramData := &dny_protocol.ParameterSettingData{}
	if err := paramData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
			"error":    err.Error(),
		}).Error("参数设置数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceId,
		"parameterType": paramData.ParameterType,
		"parameterId":   paramData.ParameterID,
		"valueLength":   len(paramData.Value),
	}).Info("收到参数设置请求")

	// 调用业务层处理参数设置
	deviceService := app.GetServiceManager().DeviceService
	success, resultValue := deviceService.HandleParameterSetting(deviceId, paramData)

	// 构建响应数据
	responseData := make([]byte, 0, 100)
	// 参数类型 (1字节)
	responseData = append(responseData, paramData.ParameterType)
	// 参数ID (2字节, 小端序)
	paramIdBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(paramIdBytes, paramData.ParameterID)
	responseData = append(responseData, paramIdBytes...)

	// 结果状态 (1字节)
	if success {
		responseData = append(responseData, dny_protocol.ResponseSuccess)
	} else {
		responseData = append(responseData, dny_protocol.ResponseFailed)
	}

	// 返回值 (变长)
	if len(resultValue) > 0 {
		responseData = append(responseData, resultValue...)
	}

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdParamSetting), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"deviceId":    deviceId,
			"parameterId": paramData.ParameterID,
			"error":       err.Error(),
		}).Error("发送参数设置响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"deviceId":    deviceId,
		"parameterId": paramData.ParameterID,
		"success":     success,
	}).Debug("参数设置响应发送成功")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理参数设置
func (h *ParameterSettingHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("参数设置处理完成")
}
