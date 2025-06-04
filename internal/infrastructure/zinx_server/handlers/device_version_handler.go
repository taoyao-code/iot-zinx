package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// DeviceVersionHandler 处理设备版本上传请求 (命令ID: 0x35)
type DeviceVersionHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理
func (h *DeviceVersionHandler) PreHandle(request ziface.IRequest) {
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备版本上传请求")
}

// Handle 处理设备版本上传请求
func (h *DeviceVersionHandler) Handle(request ziface.IRequest) {
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 设备版本处理器：开始处理标准Zinx消息")

	// 获取PhysicalID
	var physicalId uint32
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		fmt.Printf("🔧 从DNY协议消息获取PhysicalID: 0x%08X\n", physicalId)
	} else if prop, err := conn.GetProperty(protocol.PROP_DNY_PHYSICAL_ID); err == nil {
		if pid, ok := prop.(uint32); ok {
			physicalId = pid
			fmt.Printf("🔧 从连接属性获取PhysicalID: 0x%08X\n", physicalId)
		}
	}

	if physicalId == 0 {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("无法获取PhysicalID，设备版本上传处理失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
	}).Info("设备版本处理器：处理标准Zinx数据格式")

	// 解析设备版本数据
	if len(data) < 9 { // 最小数据长度：端口数(1) + 设备类型(1) + 版本号(2) + 物理ID(4) + ...
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
			"dataHex":    hex.EncodeToString(data),
		}).Error("设备版本数据长度不足")

		// 发送错误响应
		responseData := []byte{dny_protocol.ResponseFailed}
		messageID := uint16(time.Now().Unix() & 0xFFFF)
		pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, 0x35, responseData)
		return
	}

	// 解析数据字段
	slaveCount := data[0]                                    // 分机数量
	deviceType := data[1]                                    // 设备类型
	version := binary.LittleEndian.Uint16(data[2:4])         // 版本号
	slavePhysicalID := binary.LittleEndian.Uint32(data[4:8]) // 分机物理ID

	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalId":      fmt.Sprintf("0x%08X", physicalId),
		"slaveCount":      slaveCount,
		"deviceType":      deviceType,
		"version":         version,
		"versionStr":      fmt.Sprintf("V%d.%02d", version/100, version%100),
		"slavePhysicalID": fmt.Sprintf("0x%08X", slavePhysicalID),
	}).Info("设备版本信息解析成功")

	// 构建响应数据
	responseData := []byte{dny_protocol.ResponseSuccess}

	// 发送响应
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, 0x35, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送设备版本响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
	}).Info("设备版本上传处理完成")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理
func (h *DeviceVersionHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("设备版本上传请求处理完成")
}
