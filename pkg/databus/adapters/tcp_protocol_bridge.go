package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// TCPProtocolBridge TCP协议桥接器
// 负责连接TCP协议处理与DataBus，实现协议数据的统一管理和分发
type TCPProtocolBridge struct {
	dataBus               databus.DataBus
	eventPublisher        databus.EventPublisher
	sessionManager        *TCPSessionManager
	eventPublisherAdapter *TCPEventPublisher

	// 协议处理
	protocolHandlers map[uint8]ProtocolHandler
	handlerMutex     sync.RWMutex

	// 配置
	config  *TCPProtocolBridgeConfig
	enabled bool

	// 统计
	stats      *ProtocolBridgeStats
	statsMutex sync.RWMutex
}

// TCPProtocolBridgeConfig TCP协议桥接器配置
type TCPProtocolBridgeConfig struct {
	EnableProtocolValidation bool          `json:"enable_protocol_validation"`
	EnableDataLogging        bool          `json:"enable_data_logging"`
	EnableMetrics            bool          `json:"enable_metrics"`
	EnableAutoRegistration   bool          `json:"enable_auto_registration"`
	ProcessingTimeout        time.Duration `json:"processing_timeout"`
	MaxPayloadSize           int           `json:"max_payload_size"`
}

// ProtocolHandler 协议处理器接口
type ProtocolHandler interface {
	HandleProtocolData(ctx context.Context, frame *protocol.DecodedDNYFrame, conn ziface.IConnection, session *TCPSession) error
	GetCommandID() uint8
	GetHandlerName() string
}

// ProtocolBridgeStats 协议桥接器统计
type ProtocolBridgeStats struct {
	TotalMessages      int64 `json:"total_messages"`
	SuccessfulMessages int64 `json:"successful_messages"`
	FailedMessages     int64 `json:"failed_messages"`
	InvalidMessages    int64 `json:"invalid_messages"`
	UnknownCommands    int64 `json:"unknown_commands"`
	ProcessingErrors   int64 `json:"processing_errors"`
	DataBusPublished   int64 `json:"databus_published"`
	DataBusErrors      int64 `json:"databus_errors"`

	// 按命令统计
	CommandStats map[uint8]*CommandStats `json:"command_stats"`

	// 时间统计
	LastMessageTime       time.Time     `json:"last_message_time"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
}

// CommandStats 命令统计
type CommandStats struct {
	Count             int64         `json:"count"`
	SuccessCount      int64         `json:"success_count"`
	ErrorCount        int64         `json:"error_count"`
	LastProcessed     time.Time     `json:"last_processed"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
}

// NewTCPProtocolBridge 创建TCP协议桥接器
func NewTCPProtocolBridge(dataBus databus.DataBus, eventPublisher databus.EventPublisher, sessionManager *TCPSessionManager, config *TCPProtocolBridgeConfig) *TCPProtocolBridge {
	if config == nil {
		config = &TCPProtocolBridgeConfig{
			EnableProtocolValidation: true,
			EnableDataLogging:        true,
			EnableMetrics:            true,
			EnableAutoRegistration:   true,
			ProcessingTimeout:        30 * time.Second,
			MaxPayloadSize:           4096,
		}
	}

	bridge := &TCPProtocolBridge{
		dataBus:          dataBus,
		eventPublisher:   eventPublisher,
		sessionManager:   sessionManager,
		protocolHandlers: make(map[uint8]ProtocolHandler),
		config:           config,
		enabled:          true,
		stats: &ProtocolBridgeStats{
			CommandStats: make(map[uint8]*CommandStats),
		},
	}

	// 创建事件发布适配器
	bridge.eventPublisherAdapter = NewTCPEventPublisher(dataBus, eventPublisher, nil)

	// 注册默认协议处理器
	bridge.registerDefaultHandlers()

	return bridge
}

// ProcessIncomingData 处理入站数据
func (b *TCPProtocolBridge) ProcessIncomingData(ctx context.Context, conn ziface.IConnection, data []byte) error {
	if !b.enabled {
		return nil
	}

	connID := conn.GetConnID()
	startTime := time.Now()

	// 更新统计
	b.updateStats(func(stats *ProtocolBridgeStats) {
		stats.TotalMessages++
		stats.LastMessageTime = startTime
	})

	// 更新会话活动
	if b.sessionManager != nil {
		if err := b.sessionManager.UpdateSessionActivity(connID, "message"); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id": connID,
				"error":   err.Error(),
			}).Debug("更新会话活动失败")
		}
	}

	// 解析协议数据
	decodedFrame, err := b.parseProtocolData(data)
	if err != nil {
		b.updateStats(func(stats *ProtocolBridgeStats) {
			stats.InvalidMessages++
		})

		logger.WithFields(logrus.Fields{
			"conn_id":  connID,
			"data_len": len(data),
			"error":    err.Error(),
		}).Error("协议数据解析失败")

		return fmt.Errorf("failed to parse protocol data: %w", err)
	}

	// 验证协议数据
	if b.config.EnableProtocolValidation {
		if err := b.validateProtocolData(decodedFrame); err != nil {
			b.updateStats(func(stats *ProtocolBridgeStats) {
				stats.InvalidMessages++
			})

			return fmt.Errorf("protocol validation failed: %w", err)
		}
	}

	// 获取会话信息
	session, exists := b.sessionManager.GetSession(connID)
	if !exists {
		logger.WithField("conn_id", connID).Warn("未找到对应会话，创建临时会话")
		// 可以选择创建临时会话或返回错误
	}

	// 创建协议数据对象
	protocolData := &databus.ProtocolData{
		ConnID:      connID,
		DeviceID:    decodedFrame.DeviceID,
		Direction:   "inbound",
		RawBytes:    data,
		Command:     decodedFrame.Command,
		MessageID:   decodedFrame.MessageID,
		Payload:     decodedFrame.Payload,
		ParsedData:  make(map[string]interface{}), // 临时创建空map
		Timestamp:   startTime,
		ProcessedAt: time.Now(),
		Status:      "processing",
		Version:     1,
	}

	// 发布协议数据到DataBus
	if err := b.dataBus.PublishProtocolData(ctx, connID, protocolData); err != nil {
		b.updateStats(func(stats *ProtocolBridgeStats) {
			stats.DataBusErrors++
		})

		logger.WithFields(logrus.Fields{
			"conn_id":   connID,
			"device_id": decodedFrame.DeviceID,
			"command":   decodedFrame.Command,
			"error":     err.Error(),
		}).Error("发布协议数据到DataBus失败")
	} else {
		b.updateStats(func(stats *ProtocolBridgeStats) {
			stats.DataBusPublished++
		})
	}

	// 查找协议处理器
	handler, exists := b.getProtocolHandler(decodedFrame.Command)
	if !exists {
		b.updateStats(func(stats *ProtocolBridgeStats) {
			stats.UnknownCommands++
		})

		logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"command": decodedFrame.Command,
		}).Warn("未找到对应的协议处理器")

		// 发布未知命令事件
		if err := b.eventPublisherAdapter.PublishProtocolEvent("unknown_command", connID, decodedFrame.DeviceID, protocolData); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("发布未知命令事件失败")
		}

		protocolData.Status = "unknown_command"
	} else {
		// 处理协议数据
		processingCtx, cancel := context.WithTimeout(ctx, b.config.ProcessingTimeout)
		defer cancel()

		if err := handler.HandleProtocolData(processingCtx, decodedFrame, conn, session); err != nil {
			b.updateStats(func(stats *ProtocolBridgeStats) {
				stats.ProcessingErrors++
				stats.FailedMessages++
			})

			protocolData.Status = "failed"
			protocolData.ProcessedAt = time.Now()

			logger.WithFields(logrus.Fields{
				"conn_id":   connID,
				"device_id": decodedFrame.DeviceID,
				"command":   decodedFrame.Command,
				"handler":   handler.GetHandlerName(),
				"error":     err.Error(),
			}).Error("协议处理失败")

			// 发布处理失败事件
			if publishErr := b.eventPublisherAdapter.PublishProtocolEvent("processing_failed", connID, decodedFrame.DeviceID, protocolData); publishErr != nil {
				logger.WithFields(logrus.Fields{
					"error": publishErr.Error(),
				}).Error("发布处理失败事件失败")
			}
		} else {
			b.updateStats(func(stats *ProtocolBridgeStats) {
				stats.SuccessfulMessages++
			})

			protocolData.Status = "completed"
			protocolData.ProcessedAt = time.Now()

			// 发布处理成功事件
			if publishErr := b.eventPublisherAdapter.PublishProtocolEvent("processing_completed", connID, decodedFrame.DeviceID, protocolData); publishErr != nil {
				logger.WithFields(logrus.Fields{
					"error": publishErr.Error(),
				}).Error("发布处理成功事件失败")
			}
		}
	}

	// 更新协议数据状态
	protocolData.ProcessedAt = time.Now()
	// 注意：DataBus接口中没有UpdateProtocolData方法，这里跳过更新

	// 更新命令统计
	processingTime := time.Since(startTime)
	b.updateCommandStats(decodedFrame.Command, processingTime, err == nil)

	if b.config.EnableDataLogging {
		logger.WithFields(logrus.Fields{
			"conn_id":         connID,
			"device_id":       decodedFrame.DeviceID,
			"command":         decodedFrame.Command,
			"message_id":      decodedFrame.MessageID,
			"payload_size":    len(decodedFrame.Payload),
			"processing_time": processingTime,
			"status":          protocolData.Status,
		}).Debug("协议数据处理完成")
	}

	return nil
}

// ProcessOutgoingData 处理出站数据
func (b *TCPProtocolBridge) ProcessOutgoingData(ctx context.Context, conn ziface.IConnection, data []byte) error {
	if !b.enabled {
		return nil
	}

	connID := conn.GetConnID()

	// 获取设备ID
	var deviceID string
	if session, exists := b.sessionManager.GetSession(connID); exists {
		deviceID = session.DeviceID
	}

	// 创建出站协议数据
	protocolData := &databus.ProtocolData{
		ConnID:      connID,
		DeviceID:    deviceID,
		Direction:   "outbound",
		RawBytes:    data,
		Timestamp:   time.Now(),
		ProcessedAt: time.Now(),
		Status:      "sent",
		Version:     1,
	}

	// 发布到DataBus
	if err := b.dataBus.PublishProtocolData(ctx, connID, protocolData); err != nil {
		logger.WithFields(logrus.Fields{
			"conn_id":   connID,
			"device_id": deviceID,
			"data_len":  len(data),
			"error":     err.Error(),
		}).Error("发布出站协议数据失败")
		return err
	}

	// 发布出站数据事件
	if err := b.eventPublisherAdapter.PublishDataEvent("data_sent", connID, deviceID, map[string]interface{}{
		"data_size": len(data),
		"timestamp": time.Now(),
	}); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("发布出站数据事件失败")
	}

	return nil
}

// RegisterProtocolHandler 注册协议处理器
func (b *TCPProtocolBridge) RegisterProtocolHandler(handler ProtocolHandler) {
	b.handlerMutex.Lock()
	defer b.handlerMutex.Unlock()

	commandID := handler.GetCommandID()
	b.protocolHandlers[commandID] = handler

	logger.WithFields(logrus.Fields{
		"command_id":   commandID,
		"handler_name": handler.GetHandlerName(),
	}).Info("协议处理器已注册")
}

// parseProtocolData 解析协议数据
func (b *TCPProtocolBridge) parseProtocolData(data []byte) (*protocol.DecodedDNYFrame, error) {
	// 基本数据验证
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	if b.config.MaxPayloadSize > 0 && len(data) > b.config.MaxPayloadSize {
		return nil, fmt.Errorf("payload too large: %d > %d", len(data), b.config.MaxPayloadSize)
	}

	// 解析DNY协议
	message, err := protocol.ParseDNYProtocolData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DNY protocol: %w", err)
	}

	// 创建解码帧
	frame := &protocol.DecodedDNYFrame{
		DeviceID:      fmt.Sprintf("%08X", message.GetPhysicalId()),
		Command:       uint8(message.GetMsgID()),
		MessageID:     uint16(message.GetDataLen()),
		Payload:       message.GetData(),
		RawData:       data,
		RawPhysicalID: data[4:8], // 假设物理ID在4-8字节
	}

	return frame, nil
}

// validateProtocolData 验证协议数据
func (b *TCPProtocolBridge) validateProtocolData(frame *protocol.DecodedDNYFrame) error {
	if frame == nil {
		return fmt.Errorf("frame is nil")
	}

	if frame.DeviceID == "" {
		return fmt.Errorf("device ID is empty")
	}

	if len(frame.Payload) == 0 {
		return fmt.Errorf("payload is empty")
	}

	// 注意：DecodedDNYFrame没有ValidationStatus字段，跳过设置
	return nil
}

// getProtocolHandler 获取协议处理器
func (b *TCPProtocolBridge) getProtocolHandler(commandID uint8) (ProtocolHandler, bool) {
	b.handlerMutex.RLock()
	defer b.handlerMutex.RUnlock()

	handler, exists := b.protocolHandlers[commandID]
	return handler, exists
}

// registerDefaultHandlers 注册默认协议处理器
func (b *TCPProtocolBridge) registerDefaultHandlers() {
	// 这里可以注册一些基本的协议处理器
	// 例如：心跳、设备注册等
	logger.Info("注册默认协议处理器")
}

// updateStats 更新统计信息
func (b *TCPProtocolBridge) updateStats(updateFunc func(*ProtocolBridgeStats)) {
	if !b.config.EnableMetrics {
		return
	}

	b.statsMutex.Lock()
	defer b.statsMutex.Unlock()
	updateFunc(b.stats)
}

// updateCommandStats 更新命令统计
func (b *TCPProtocolBridge) updateCommandStats(commandID uint8, processingTime time.Duration, success bool) {
	if !b.config.EnableMetrics {
		return
	}

	b.statsMutex.Lock()
	defer b.statsMutex.Unlock()

	cmdStats, exists := b.stats.CommandStats[commandID]
	if !exists {
		cmdStats = &CommandStats{}
		b.stats.CommandStats[commandID] = cmdStats
	}

	cmdStats.Count++
	cmdStats.LastProcessed = time.Now()

	if success {
		cmdStats.SuccessCount++
	} else {
		cmdStats.ErrorCount++
	}

	// 计算平均处理时间
	if cmdStats.Count == 1 {
		cmdStats.AvgProcessingTime = processingTime
	} else {
		cmdStats.AvgProcessingTime = (cmdStats.AvgProcessingTime*time.Duration(cmdStats.Count-1) + processingTime) / time.Duration(cmdStats.Count)
	}
}

// GetStats 获取统计信息
func (b *TCPProtocolBridge) GetStats() *ProtocolBridgeStats {
	b.statsMutex.RLock()
	defer b.statsMutex.RUnlock()

	// 手动复制统计信息以避免锁复制
	statsCopy := &ProtocolBridgeStats{
		TotalMessages:         b.stats.TotalMessages,
		SuccessfulMessages:    b.stats.SuccessfulMessages,
		FailedMessages:        b.stats.FailedMessages,
		InvalidMessages:       b.stats.InvalidMessages,
		UnknownCommands:       b.stats.UnknownCommands,
		ProcessingErrors:      b.stats.ProcessingErrors,
		DataBusPublished:      b.stats.DataBusPublished,
		DataBusErrors:         b.stats.DataBusErrors,
		LastMessageTime:       b.stats.LastMessageTime,
		AverageProcessingTime: b.stats.AverageProcessingTime,
		CommandStats:          make(map[uint8]*CommandStats),
	}

	// 复制命令统计
	for k, v := range b.stats.CommandStats {
		cmdStatsCopy := *v
		statsCopy.CommandStats[k] = &cmdStatsCopy
	}

	return statsCopy
}

// Enable 启用桥接器
func (b *TCPProtocolBridge) Enable() {
	b.enabled = true
	logger.Info("TCP协议桥接器已启用")
}

// Disable 禁用桥接器
func (b *TCPProtocolBridge) Disable() {
	b.enabled = false
	logger.Info("TCP协议桥接器已禁用")
}

// IsEnabled 检查是否启用
func (b *TCPProtocolBridge) IsEnabled() bool {
	return b.enabled
}

// Stop 停止桥接器
func (b *TCPProtocolBridge) Stop() {
	b.enabled = false

	if b.eventPublisherAdapter != nil {
		b.eventPublisherAdapter.Stop()
	}

	logger.Info("TCP协议桥接器已停止")
}

// GetMetrics 获取指标
func (b *TCPProtocolBridge) GetMetrics() map[string]interface{} {
	stats := b.GetStats()

	metrics := map[string]interface{}{
		"enabled":                 b.enabled,
		"total_messages":          stats.TotalMessages,
		"successful_messages":     stats.SuccessfulMessages,
		"failed_messages":         stats.FailedMessages,
		"invalid_messages":        stats.InvalidMessages,
		"unknown_commands":        stats.UnknownCommands,
		"processing_errors":       stats.ProcessingErrors,
		"databus_published":       stats.DataBusPublished,
		"databus_errors":          stats.DataBusErrors,
		"last_message_time":       stats.LastMessageTime,
		"average_processing_time": stats.AverageProcessingTime.String(),
		"registered_handlers":     len(b.protocolHandlers),
		"config":                  b.config,
	}

	// 添加命令统计
	commandMetrics := make(map[string]interface{})
	for cmdID, cmdStats := range stats.CommandStats {
		commandMetrics[fmt.Sprintf("cmd_%d", cmdID)] = map[string]interface{}{
			"count":               cmdStats.Count,
			"success_count":       cmdStats.SuccessCount,
			"error_count":         cmdStats.ErrorCount,
			"last_processed":      cmdStats.LastProcessed,
			"avg_processing_time": cmdStats.AvgProcessingTime.String(),
		}
	}
	metrics["command_stats"] = commandMetrics

	return metrics
}
