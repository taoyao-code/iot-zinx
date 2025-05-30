package network

import (
	"github.com/aceld/zinx/ziface"
)

// PacketHandler 定义了数据包处理器接口
type PacketHandler interface {
	// HandlePacket 处理接收到的数据包
	HandlePacket(conn ziface.IConnection, data []byte) bool
}

// ICommandManager 定义了命令管理器接口
type ICommandManager interface {
	// Start 启动命令管理器
	Start()

	// Stop 停止命令管理器
	Stop()

	// RegisterCommand 注册命令
	RegisterCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte)

	// ConfirmCommand 确认命令已完成
	ConfirmCommand(physicalID uint32, messageID uint16, command uint8) bool
}

// IConnectionHooks 定义了连接钩子接口
type IConnectionHooks interface {
	// OnConnectionStart 当连接建立时的钩子函数
	OnConnectionStart(conn ziface.IConnection)

	// OnConnectionStop 当连接断开时的钩子函数
	OnConnectionStop(conn ziface.IConnection)

	// SetOnConnectionEstablishedFunc 设置连接建立回调函数
	SetOnConnectionEstablishedFunc(fn func(conn ziface.IConnection))

	// SetOnConnectionClosedFunc 设置连接关闭回调函数
	SetOnConnectionClosedFunc(fn func(conn ziface.IConnection))
}
