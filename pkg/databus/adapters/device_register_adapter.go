package adapters

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// DeviceRegisterAdapter 设备注册适配器
// 使用协议数据适配器重构的设备注册处理逻辑
type DeviceRegisterAdapter struct {
	protocolAdapter *ProtocolDataAdapter
	logger          *logrus.Entry
}

// NewDeviceRegisterAdapter 创建设备注册适配器
func NewDeviceRegisterAdapter(dataBus databus.DataBus) *DeviceRegisterAdapter {
	return &DeviceRegisterAdapter{
		protocolAdapter: NewProtocolDataAdapter(dataBus),
		logger:          logger.WithField("component", "DeviceRegisterAdapter"),
	}
}

// HandleRequest 处理设备注册请求
func (adapter *DeviceRegisterAdapter) HandleRequest(request ziface.IRequest) error {
	conn := request.GetConnection()

	// 🔧 增加安全检查，防止空指针
	if conn == nil {
		adapter.logger.Error("连接对象为空")
		return fmt.Errorf("连接对象为空")
	}

	// 从请求中提取协议消息
	msg, err := adapter.extractProtocolMessage(request)
	if err != nil {
		adapter.logger.WithFields(logrus.Fields{
			"conn_id": conn.GetConnID(),
			"error":   err.Error(),
		}).Error("提取协议消息失败")
		return err
	}

	// 🔧 增加协议消息安全检查
	if msg == nil {
		adapter.logger.WithField("conn_id", conn.GetConnID()).Error("协议消息为空")
		return fmt.Errorf("协议消息为空")
	}

	// 使用协议数据适配器处理消息
	result, err := adapter.protocolAdapter.ProcessProtocolMessage(msg, conn)
	if err != nil {
		adapter.logger.WithFields(logrus.Fields{
			"conn_id": conn.GetConnID(),
			"error":   err.Error(),
		}).Error("协议消息处理失败")
		return err
	}

	// 🔧 增加结果安全检查
	if result == nil {
		adapter.logger.WithField("conn_id", conn.GetConnID()).Error("协议处理结果为空")
		return fmt.Errorf("协议处理结果为空")
	}

	// 发送响应（如果需要）
	if result.ShouldRespond && len(result.ResponseData) > 0 {
		if err := adapter.sendResponse(conn, msg, result.ResponseData); err != nil {
			adapter.logger.WithFields(logrus.Fields{
				"conn_id": conn.GetConnID(),
				"error":   err.Error(),
			}).Error("发送响应失败")
			return err
		}
	}

	// 处理通知（如果需要）
	if result.RequiresNotification {
		adapter.handleNotification(result.NotificationData)
	}

	adapter.logger.WithFields(logrus.Fields{
		"conn_id": conn.GetConnID(),
		"success": result.Success,
		"message": result.Message,
	}).Info("设备注册处理完成")

	return nil
}

// extractProtocolMessage 从Zinx请求中提取协议消息
func (adapter *DeviceRegisterAdapter) extractProtocolMessage(request ziface.IRequest) (*dny_protocol.Message, error) {
	// 1. 尝试从责任链的附加数据中获取已解析的 *dny_protocol.Message
	// DNY_Decoder应该通过chain.ProceedWithIMessage传递解码后的统一消息对象
	if attachedData := request.GetResponse(); attachedData != nil {
		if unifiedMsg, ok := attachedData.(*dny_protocol.Message); ok {
			return unifiedMsg, nil
		}
	}

	// 2. 如果没有找到附加数据，尝试直接解析数据
	rawData := request.GetData()
	if len(rawData) == 0 {
		return nil, fmt.Errorf("请求数据为空")
	}

	// 使用协议解析器解析数据
	msg, err := protocol.ParseDNYProtocolData(rawData)
	if err != nil {
		return nil, fmt.Errorf("协议解析失败: %v", err)
	}

	return msg, nil
}

// sendResponse 发送响应
func (adapter *DeviceRegisterAdapter) sendResponse(conn ziface.IConnection, originalMsg *dny_protocol.Message, responseData []byte) error {
	// 对于设备注册，使用DNY协议格式发送响应
	if originalMsg.MessageType == "standard" && originalMsg.CommandId == constants.CmdDeviceRegister {
		return protocol.SendDNYResponse(
			conn,
			originalMsg.PhysicalId,
			originalMsg.MessageId,
			constants.CmdDeviceRegister,
			responseData,
		)
	}

	// 对于其他消息类型，直接发送数据
	return conn.SendMsg(uint32(originalMsg.CommandId), responseData)
}

// handleNotification 处理通知
func (adapter *DeviceRegisterAdapter) handleNotification(notificationData map[string]interface{}) {
	if notificationData == nil {
		return
	}

	adapter.logger.WithFields(logrus.Fields{
		"notification_data": notificationData,
	}).Debug("处理设备注册通知")

	// TODO: 实现具体的通知逻辑
	// 可以集成现有的notification系统
}

// GetStats 获取适配器统计信息
func (adapter *DeviceRegisterAdapter) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"adapter_type":     "device_register_adapter",
		"protocol_adapter": adapter.protocolAdapter.GetStats(),
	}
	return stats
}
