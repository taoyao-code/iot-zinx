package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// UnifiedDataSender 统一数据发送器
// 这是所有下行命令的唯一出口，通过设备组管理器实现对串联设备的精确发送
type UnifiedDataSender struct {
	groupManager     *core.ConnectionGroupManager
	messageIDCounter uint16
	mutex            sync.RWMutex
	stats            *SenderStats
}

// SenderStats 发送统计信息
type SenderStats struct {
	TotalSent     int64     `json:"totalSent"`
	SuccessCount  int64     `json:"successCount"`
	FailureCount  int64     `json:"failureCount"`
	LastSentTime  time.Time `json:"lastSentTime"`
	LastErrorTime time.Time `json:"lastErrorTime"`
	LastError     string    `json:"lastError"`
	mutex         sync.RWMutex
}

// SendResult 发送结果
type SendResult struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	MessageID uint16    `json:"messageId"`
	ConnID    uint64    `json:"connId"`
	Timestamp time.Time `json:"timestamp"`
}

// 全局统一发送器实例
var (
	globalUnifiedSender     *UnifiedDataSender
	globalUnifiedSenderOnce sync.Once
)

// GetGlobalUnifiedSender 获取全局统一发送器实例
func GetGlobalUnifiedSender() *UnifiedDataSender {
	globalUnifiedSenderOnce.Do(func() {
		globalUnifiedSender = &UnifiedDataSender{
			groupManager:     core.GetGlobalConnectionGroupManager(),
			messageIDCounter: 1,
			stats:            &SenderStats{},
		}
	})
	return globalUnifiedSender
}

// SendDataToDevice 向指定设备发送数据
// 这是所有下行命令的统一入口点
func (s *UnifiedDataSender) SendDataToDevice(deviceID string, commandID uint8, payload []byte) (*SendResult, error) {
	startTime := time.Now()

	// 记录发送日志
	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"commandID":  fmt.Sprintf("0x%02X", commandID),
		"payloadLen": len(payload),
	}).Info("[SEND] 准备发送数据到设备")

	// 1. 通过设备组管理器查找设备所属的TCP连接
	conn, exists := s.groupManager.GetConnectionByDeviceID(deviceID)
	if !exists {
		err := fmt.Errorf("设备 %s 不在线或未注册", deviceID)
		s.updateStats(false, err.Error())

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Error("[SEND] 设备查找失败")

		return &SendResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: startTime,
		}, err
	}

	// 2. 解析设备ID为物理ID
	physicalID, err := s.parseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		s.updateStats(false, err.Error())

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Error("[SEND] 设备ID解析失败")

		return &SendResult{
			Success:   false,
			Error:     err.Error(),
			ConnID:    conn.GetConnID(),
			Timestamp: startTime,
		}, err
	}

	// 3. 生成消息ID
	messageID := s.getNextMessageID()

	// 4. 构建并发送DNY协议数据 - 🔧 使用统一DNY构建器
	packet := protocol.BuildUnifiedDNYPacket(physicalID, messageID, commandID, payload)

	// 🔧 修复：使用统一发送器
	globalSender := network.GetGlobalSender()
	if globalSender == nil {
		return &SendResult{
			Success:   false,
			ConnID:    conn.GetConnID(),
			Timestamp: startTime,
		}, fmt.Errorf("统一发送器未初始化")
	}

	err = globalSender.SendDNYPacket(conn, packet)
	if err != nil {
		s.updateStats(false, err.Error())

		logger.WithFields(logrus.Fields{
			"deviceID":   deviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"commandID":  fmt.Sprintf("0x%02X", commandID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"connID":     conn.GetConnID(),
			"error":      err.Error(),
		}).Error("[SEND] 数据发送失败")

		return &SendResult{
			Success:   false,
			Error:     err.Error(),
			MessageID: messageID,
			ConnID:    conn.GetConnID(),
			Timestamp: startTime,
		}, err
	}

	// 5. 发送成功
	s.updateStats(true, "")

	// 记录到通信日志
	logger.LogSendData(deviceID, commandID, messageID, conn.GetConnID(), len(payload), "命令发送")

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"commandID":  fmt.Sprintf("0x%02X", commandID),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"connID":     conn.GetConnID(),
		"payloadLen": len(payload),
		"duration":   time.Since(startTime),
	}).Info("[SEND] 数据发送成功")

	return &SendResult{
		Success:   true,
		MessageID: messageID,
		ConnID:    conn.GetConnID(),
		Timestamp: startTime,
	}, nil
}

// SendCommandToDevice 发送命令到设备（带命令描述的便捷方法）
func (s *UnifiedDataSender) SendCommandToDevice(deviceID string, commandID uint8, payload []byte, description string) (*SendResult, error) {
	logger.WithFields(logrus.Fields{
		"deviceID":    deviceID,
		"commandID":   fmt.Sprintf("0x%02X", commandID),
		"description": description,
		"payloadLen":  len(payload),
	}).Info("[SEND] 发送命令到设备")

	return s.SendDataToDevice(deviceID, commandID, payload)
}

// parseDeviceIDToPhysicalID 解析设备ID字符串为物理ID
func (s *UnifiedDataSender) parseDeviceIDToPhysicalID(deviceID string) (uint32, error) {
	var physicalID uint32

	// 尝试解析为16进制
	_, err := fmt.Sscanf(deviceID, "%X", &physicalID)
	if err != nil {
		// 如果16进制解析失败，尝试直接解析为数字
		_, err2 := fmt.Sscanf(deviceID, "%d", &physicalID)
		if err2 != nil {
			return 0, fmt.Errorf("设备ID格式错误，应为16进制或10进制数字: %s", deviceID)
		}
	}

	return physicalID, nil
}

// updateStats 更新发送统计信息
func (s *UnifiedDataSender) updateStats(success bool, errorMsg string) {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()

	s.stats.TotalSent++
	s.stats.LastSentTime = time.Now()

	if success {
		s.stats.SuccessCount++
	} else {
		s.stats.FailureCount++
		s.stats.LastErrorTime = time.Now()
		s.stats.LastError = errorMsg
	}
}

// GetStats 获取发送统计信息
func (s *UnifiedDataSender) GetStats() *SenderStats {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	// 返回副本，避免并发问题
	return &SenderStats{
		TotalSent:     s.stats.TotalSent,
		SuccessCount:  s.stats.SuccessCount,
		FailureCount:  s.stats.FailureCount,
		LastSentTime:  s.stats.LastSentTime,
		LastErrorTime: s.stats.LastErrorTime,
		LastError:     s.stats.LastError,
	}
}

// GetSuccessRate 获取发送成功率
func (s *UnifiedDataSender) GetSuccessRate() float64 {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	if s.stats.TotalSent == 0 {
		return 0.0
	}

	return float64(s.stats.SuccessCount) / float64(s.stats.TotalSent) * 100.0
}

// IsDeviceOnline 检查设备是否在线
func (s *UnifiedDataSender) IsDeviceOnline(deviceID string) bool {
	_, exists := s.groupManager.GetConnectionByDeviceID(deviceID)
	return exists
}

// GetDeviceConnection 获取设备连接信息
func (s *UnifiedDataSender) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
	return s.groupManager.GetConnectionByDeviceID(deviceID)
}

// getNextMessageID 生成下一个消息ID
func (s *UnifiedDataSender) getNextMessageID() uint16 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.messageIDCounter++
	if s.messageIDCounter == 0 {
		s.messageIDCounter = 1 // 避免使用0作为消息ID
	}

	return s.messageIDCounter
}

// buildDNYPacket 构建DNY协议数据包 (已废弃)
// 🔧 重构：此函数已废弃，使用统一DNY构建器替代
func (s *UnifiedDataSender) buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	logger.WithFields(logrus.Fields{
		"function": "UnifiedDataSender.buildDNYPacket",
		"note":     "已废弃，使用统一DNY构建器",
	}).Debug("废弃函数调用")

	return protocol.BuildUnifiedDNYPacket(physicalID, messageID, command, data)
}

// calculateChecksum 计算DNY协议校验和 (已废弃)
// 🔧 重构：此函数已废弃，校验和计算已集成到统一DNY构建器中
func (s *UnifiedDataSender) calculateChecksum(data []byte) uint16 {
	logger.WithFields(logrus.Fields{
		"function": "UnifiedDataSender.calculateChecksum",
		"note":     "已废弃，校验和计算已集成到统一DNY构建器",
	}).Debug("废弃函数调用")

	// 使用统一构建器的校验和计算
	builder := protocol.GetGlobalDNYBuilder()
	return builder.CalculateChecksum(data)
}
