package protocol

import (
	"sync/atomic"
)

// MessageIDManager 全局消息ID管理器
// 提供线程安全的递增消息ID生成，符合DNY协议规范
type MessageIDManager struct {
	counter uint32
}

// 全局消息ID管理器实例
var globalMessageIDManager = &MessageIDManager{
	counter: 0, // 从0开始，GetNextMessageID()会返回1作为第一个ID
}

// GetNextMessageID 获取下一个消息ID
// 返回范围：1-65535（避免使用0作为消息ID）
// 线程安全：使用atomic操作确保并发安全
func GetNextMessageID() uint16 {
	// 原子递增计数器
	newValue := atomic.AddUint32(&globalMessageIDManager.counter, 1)

	// 转换为uint16，如果超过65535则从1重新开始
	// 避免使用0作为消息ID，符合协议要求
	messageID := uint16(newValue)
	if messageID == 0 {
		// 如果计数器溢出导致messageID为0，重置计数器并返回1
		atomic.StoreUint32(&globalMessageIDManager.counter, 1)
		return 1
	}

	return messageID
}

// ResetMessageIDCounter 重置消息ID计数器（主要用于测试）
func ResetMessageIDCounter() {
	atomic.StoreUint32(&globalMessageIDManager.counter, 0)
}

// GetCurrentMessageID 获取当前消息ID（不递增，主要用于调试）
func GetCurrentMessageID() uint16 {
	current := atomic.LoadUint32(&globalMessageIDManager.counter)
	if current == 0 {
		return 0 // 还未生成过任何消息ID
	}
	return uint16(current)
}
