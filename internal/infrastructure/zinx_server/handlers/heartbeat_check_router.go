package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// HeartbeatCheckRouter 处理Zinx框架发送的心跳检测消息的响应
// 实现了Zinx的心跳检测Router接口，处理设备对心跳检测的回复
// 注意：这个处理器处理的是自定义的心跳消息ID 0xF001
type HeartbeatCheckRouter struct {
	znet.BaseRouter
}

// Handle 处理心跳检测消息的响应
func (r *HeartbeatCheckRouter) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 记录心跳响应信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"msgID":      msg.GetMsgID(),
		"dataLen":    msg.GetDataLen(),
	}).Debug("收到设备心跳检测响应")

	// 检查是否有原始DNY消息数据
	data := msg.GetData()
	// 如果消息中包含内部命令ID，则进一步处理
	if len(data) > 0 {
		// 第一个字节是内部命令ID
		innerCmdID := data[0]
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"innerCmdID": fmt.Sprintf("0x%02X", innerCmdID),
		}).Debug("心跳消息包含内部命令ID")

		// 如果是0x81，则按照设备状态查询处理
		if innerCmdID == dny_protocol.CmdNetworkStatus {
			// 尝试将消息转换为DNY消息
			dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
			if ok {
				// 如果是DNY协议消息，提取物理ID
				physicalId := dnyMsg.GetPhysicalId()
				deviceID := fmt.Sprintf("%08X", physicalId)

				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"deviceID":   deviceID,
					"physicalId": fmt.Sprintf("0x%08X", physicalId),
					"innerCmd":   "设备状态查询(0x81)",
				}).Info("处理心跳响应中的设备状态查询")

				// 如果设备ID与连接未关联，进行关联
				if val, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err != nil || val == nil {
					zinx_server.BindDeviceIdToConnection(deviceID, conn)
				}
			}
		}
	} else {
		// 非DNY协议消息，尝试获取设备ID用于日志记录
		deviceID := "unknown"
		if val, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err == nil && val != nil {
			deviceID = val.(string)
		}

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   deviceID,
			"remoteAddr": conn.RemoteAddr().String(),
			"msgID":      msg.GetMsgID(),
		}).Debug("收到简单心跳响应")
	}

	// 无论是什么类型的响应，都更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 获取设备ID用于日志记录
	deviceID := "unknown"
	if val, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 更新设备状态为在线
	zinx_server.UpdateDeviceStatus(deviceID, "online")

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"remoteAddr": conn.RemoteAddr().String(),
		"status":     zinx_server.ConnStatusActive,
	}).Debug("心跳检测响应处理完成，设备状态已更新")
}
