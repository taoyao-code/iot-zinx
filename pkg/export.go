package pkg

import (
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// === 简化的全局变量 ===

// 简化的消息ID计数器
var messageIDCounter uint64

// 全局统一发送器实例
var globalUnifiedSender *network.UnifiedSender

// 初始化全局实例
func init() {
	globalUnifiedSender = network.NewUnifiedSender()
	// 启动统一发送器
	if err := globalUnifiedSender.Start(); err != nil {
		// 如果启动失败，记录但不阻止程序运行
		// logger会在later阶段处理错误
	}

	// 注册发送函数到protocol包（避免循环导入）
	protocol.RegisterGlobalSendDNYResponse(func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
		return globalUnifiedSender.SendDNYResponse(conn, physicalId, messageId, command, data)
	})
}

// === 核心导出接口 ===

// Core 核心模块导出
var Core = struct {
	// TCP管理器
	GetGlobalTCPManager func() *core.TCPManager
}{
	GetGlobalTCPManager: func() *core.TCPManager {
		return core.GetGlobalTCPManager()
	},
}

// Protocol 协议相关功能导出
var Protocol = struct {
	// 数据包处理
	NewDNYDataPackFactory func() protocol.IDataPackFactory
	NewDNYDecoder         func() ziface.IDecoder

	// 数据解析
	ParseDNYData      func(data []byte) (*protocol.DNYParseResult, error)
	ParseDNYHexString func(hexStr string) (*protocol.DNYParseResult, error)

	// 数据发送
	SendDNYResponse func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error

	// 消息ID管理
	GetNextMessageID func() uint16
}{
	NewDNYDataPackFactory: protocol.NewDNYDataPackFactory,
	NewDNYDecoder:         protocol.NewDNYDecoder,
	ParseDNYData:          protocol.ParseDNYData,
	ParseDNYHexString:     protocol.ParseDNYHexString,
	SendDNYResponse: func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
		// 🔧 重构：使用统一发送器替代废弃的sender.go
		return globalUnifiedSender.SendDNYResponse(conn, physicalId, messageId, command, data)
	},
	GetNextMessageID: func() uint16 {
		// 简化的消息ID生成器
		newValue := atomic.AddUint64(&messageIDCounter, 1)
		messageID := uint16(newValue % 65535)
		if messageID == 0 {
			messageID = 1
		}
		return messageID
	},
}

// === 简化的初始化函数 ===

// InitBasicArchitecture 初始化基础架构
func InitBasicArchitecture() {
	// 启动TCP管理器
	tcpManager := core.GetGlobalTCPManager()
	if err := tcpManager.Start(); err != nil {
		panic("启动TCP管理器失败: " + err.Error())
	}

	// 启动命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Start()
}

// CleanupBasicArchitecture 清理基础架构资源
func CleanupBasicArchitecture() {
	// 停止命令管理器
	cmdMgr := network.GetCommandManager()
	if cmdMgr != nil {
		cmdMgr.Stop()
	}

	// 停止TCP管理器
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		tcpManager.Stop()
	}

	// 停止统一发送器
	if globalUnifiedSender != nil {
		globalUnifiedSender.Stop()
	}
}

// === 向后兼容的发送函数（替代废弃的sender.go）===

// SendHeartbeatResponse 发送心跳响应
func SendHeartbeatResponse(conn ziface.IConnection, physicalId uint32, messageId uint16) error {
	return globalUnifiedSender.SendDNYResponse(conn, physicalId, messageId, 0x06, nil)
}

// SendRegistrationResponse 发送注册响应
func SendRegistrationResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, success bool) error {
	var data []byte
	if success {
		data = []byte{0x01} // 成功
	} else {
		data = []byte{0x00} // 失败
	}
	return globalUnifiedSender.SendDNYResponse(conn, physicalId, messageId, 0x20, data)
}

// SendTimeResponse 发送时间响应
func SendTimeResponse(conn ziface.IConnection, physicalId uint32, messageId uint16) error {
	// 获取当前时间戳（4字节，大端序）
	timestamp := uint32(time.Now().Unix())
	data := []byte{
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp & 0xFF),
	}
	return globalUnifiedSender.SendDNYResponse(conn, physicalId, messageId, 0x22, data)
}
