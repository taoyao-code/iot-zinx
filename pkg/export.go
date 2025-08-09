package pkg

import (
	"sync/atomic"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// === 简化的全局变量 ===

// 简化的消息ID计数器
var messageIDCounter uint64

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
	SendDNYResponse:       protocol.SendDNYResponse,
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
}
