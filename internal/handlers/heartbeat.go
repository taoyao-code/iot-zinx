package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// HeartbeatRouter 心跳路由器 - 优化版
type HeartbeatRouter struct {
	*BaseHandler
	connectionMonitor *ConnectionMonitor
	heartbeatManager  *HeartbeatManager
}

// NewHeartbeatRouter 创建心跳路由器
func NewHeartbeatRouter() *HeartbeatRouter {
	return &HeartbeatRouter{
		BaseHandler:      NewBaseHandler("Heartbeat"),
		heartbeatManager: NewHeartbeatManager(),
	}
}

// SetConnectionMonitor 设置连接监控器
func (r *HeartbeatRouter) SetConnectionMonitor(monitor *ConnectionMonitor) {
	r.connectionMonitor = monitor
	r.heartbeatManager.SetConnectionMonitor(monitor)
}

// PreHandle 预处理
func (r *HeartbeatRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理心跳请求 - 优化版，使用HeartbeatManager
func (r *HeartbeatRouter) Handle(request ziface.IRequest) {
	r.Log("收到心跳请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 🔧 修复：确保是心跳消息（支持新版0x21和旧版0x01）
	if err := r.ValidateMessageTypes(parsedMsg, dny_protocol.MsgTypeHeartbeat, dny_protocol.MsgTypeOldHeartbeat); err != nil {
		return
	}

	// 确定心跳类型
	heartbeatType := "standard"
	if parsedMsg.MessageType == dny_protocol.MsgTypeOldHeartbeat {
		heartbeatType = "legacy"
	}

	// 使用HeartbeatManager处理心跳
	if err := r.heartbeatManager.ProcessHeartbeat(request, heartbeatType); err != nil {
		r.Log("心跳处理失败: %v", err)
		return
	}

	// 发送心跳响应
	response := r.BuildHeartbeatResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID))
	r.SendSuccessResponse(request, response)
}

// PostHandle 后处理
func (r *HeartbeatRouter) PostHandle(request ziface.IRequest) {}
