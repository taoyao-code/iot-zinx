package handlers

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
)

// ConnectionMonitor 用于监视和记录TCP连接的数据
type ConnectionMonitor struct {
	// 日志记录器
	logger *TCPDataLogger

	// 连接映射表
	connections sync.Map

	// 是否启用
	enabled bool
}

// 全局监视器实例
var (
	globalMonitor     *ConnectionMonitor
	globalMonitorOnce sync.Once
)

// GetGlobalMonitor 获取全局监视器实例
func GetGlobalMonitor() *ConnectionMonitor {
	globalMonitorOnce.Do(func() {
		var err error
		globalMonitor = &ConnectionMonitor{
			enabled: true,
		}

		// 创建日志记录器
		globalMonitor.logger, err = NewTCPDataLogger("logs/tcp_data", true)
		if err != nil {
			fmt.Printf("创建TCP数据记录器失败: %v\n", err)
			globalMonitor.enabled = false
		} else {
			fmt.Printf("TCP数据记录器已创建，日志文件: %s\n", globalMonitor.logger.GetLogFilePath())
		}
	})

	return globalMonitor
}

// OnConnectionOpen 当连接打开时调用
func (m *ConnectionMonitor) OnConnectionOpen(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	// 记录连接信息
	remoteAddr := conn.RemoteAddr().String()
	m.connections.Store(conn.GetConnID(), remoteAddr)

	message := fmt.Sprintf("连接打开: ID=%d, 远程地址=%s", conn.GetConnID(), remoteAddr)
	fmt.Println(message)
	m.logger.LogMessage(message)
}

// OnConnectionClose 当连接关闭时调用
func (m *ConnectionMonitor) OnConnectionClose(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	// 获取连接信息
	remoteAddr, ok := m.connections.Load(conn.GetConnID())
	if !ok {
		remoteAddr = "未知"
	}

	// 记录连接关闭信息
	message := fmt.Sprintf("连接关闭: ID=%d, 远程地址=%s", conn.GetConnID(), remoteAddr)
	fmt.Println(message)
	m.logger.LogMessage(message)

	// 从映射表中删除连接
	m.connections.Delete(conn.GetConnID())
}

// OnDataReceived 当接收到数据时调用
func (m *ConnectionMonitor) OnDataReceived(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	// 获取连接信息
	remoteAddr, ok := m.connections.Load(conn.GetConnID())
	if !ok {
		remoteAddr = "未知"
	}

	// 记录接收到的数据
	m.logger.LogTCPData(data, "接收", remoteAddr.(string))

	// 解析并打印数据
	m.parseAndPrintData(data, "接收", remoteAddr.(string))
}

// OnDataSent 当发送数据时调用
func (m *ConnectionMonitor) OnDataSent(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	// 获取连接信息
	remoteAddr, ok := m.connections.Load(conn.GetConnID())
	if !ok {
		remoteAddr = "未知"
	}

	// 记录发送的数据
	m.logger.LogTCPData(data, "发送", remoteAddr.(string))

	// 解析并打印数据
	m.parseAndPrintData(data, "发送", remoteAddr.(string))
}

// parseAndPrintData 解析并打印数据
func (m *ConnectionMonitor) parseAndPrintData(data []byte, direction, remoteAddr string) {
	// 检查是否为DNY协议数据
	if len(data) >= 3 && data[0] == 0x44 && data[1] == 0x4E && data[2] == 0x59 {
		parser := &DNYProtocolParser{}
		err := parser.Parse(data)
		if err == nil {
			// 打印解析结果
			timestamp := time.Now().Format("2006-01-02 15:04:05.000")
			fmt.Printf("\n[%s] %s 数据 - %s\n", timestamp, direction, remoteAddr)
			fmt.Printf("命令: 0x%02X (%s)\n", parser.Command, parser.GetCommandName())
			fmt.Printf("物理ID: 0x%08X\n", parser.PhysicalID)
			fmt.Printf("消息ID: 0x%04X\n", parser.MessageID)
			fmt.Printf("数据长度: %d\n", len(parser.Data))
			fmt.Printf("校验结果: %v\n", parser.VerifyChecksum())
			fmt.Println("----------------------------------------")
		}
	}
}

// Close 关闭监视器
func (m *ConnectionMonitor) Close() {
	if m.enabled && m.logger != nil {
		_ = m.logger.Close()
		m.enabled = false
	}
}

// ParseManualHexData 手动解析十六进制数据
func (m *ConnectionMonitor) ParseManualHexData(hexData, description string) {
	if !m.enabled {
		return
	}

	// 记录并解析数据
	m.logger.ParseHexString(hexData, description)

	// 尝试解析DNY协议
	parser := &DNYProtocolParser{}
	err := parser.ParseHexString(hexData)
	if err == nil {
		// 打印解析结果
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Printf("\n[%s] 手动解析: %s\n", timestamp, description)
		fmt.Printf("命令: 0x%02X (%s)\n", parser.Command, parser.GetCommandName())
		fmt.Printf("物理ID: 0x%08X\n", parser.PhysicalID)
		fmt.Printf("消息ID: 0x%04X\n", parser.MessageID)
		fmt.Printf("数据长度: %d\n", len(parser.Data))
		fmt.Printf("校验结果: %v\n", parser.VerifyChecksum())
		fmt.Println("----------------------------------------")
	} else {
		fmt.Printf("\n[手动解析失败] %s: %v\n", description, err)
	}
}
