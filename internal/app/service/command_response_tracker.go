package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// CommandResponseTracker 命令响应跟踪器
type CommandResponseTracker struct {
	// 存储等待响应的命令
	pendingCommands sync.Map // map[string]*PendingCommand

	// 清理过期命令的定时器
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	mutex sync.RWMutex
}

// PendingCommand 等待响应的命令
type PendingCommand struct {
	ID         string                                  `json:"id"`
	DeviceID   string                                  `json:"deviceId"`
	Command    byte                                    `json:"command"`
	MessageID  uint16                                  `json:"messageId"`
	CreatedAt  time.Time                               `json:"createdAt"`
	Timeout    time.Duration                           `json:"timeout"`
	ResponseCh chan *dto.ChargeControlResponse         `json:"-"`
	Callback   func(*dto.ChargeControlResponse, error) `json:"-"`
	Context    context.Context                         `json:"-"`
	Cancel     context.CancelFunc                      `json:"-"`
}

// NewCommandResponseTracker 创建命令响应跟踪器
func NewCommandResponseTracker() *CommandResponseTracker {
	tracker := &CommandResponseTracker{
		cleanupTicker: time.NewTicker(30 * time.Second), // 每30秒清理一次过期命令
		stopCleanup:   make(chan struct{}),
	}

	// 启动清理协程
	go tracker.startCleanup()

	return tracker
}

// TrackCommand 跟踪命令，等待响应
func (t *CommandResponseTracker) TrackCommand(
	deviceID string,
	command byte,
	messageID uint16,
	timeout time.Duration,
	callback func(*dto.ChargeControlResponse, error),
) *PendingCommand {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// 生成命令ID
	commandID := fmt.Sprintf("%s_%02x_%04x_%d", deviceID, command, messageID, time.Now().UnixNano())

	// 创建上下文和取消函数
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// 创建等待命令
	pendingCmd := &PendingCommand{
		ID:         commandID,
		DeviceID:   deviceID,
		Command:    command,
		MessageID:  messageID,
		CreatedAt:  time.Now(),
		Timeout:    timeout,
		ResponseCh: make(chan *dto.ChargeControlResponse, 1),
		Callback:   callback,
		Context:    ctx,
		Cancel:     cancel,
	}

	// 存储等待命令
	t.pendingCommands.Store(commandID, pendingCmd)

	logger.Info("开始跟踪命令响应")

	// 启动超时处理协程
	go t.handleCommandTimeout(pendingCmd)

	return pendingCmd
}

// NotifyResponse 通知收到响应
func (t *CommandResponseTracker) NotifyResponse(deviceID string, messageID uint16, response *dto.ChargeControlResponse) bool {
	var found *PendingCommand
	var commandID string

	// 查找对应的等待命令
	t.pendingCommands.Range(func(key, value interface{}) bool {
		cmd := value.(*PendingCommand)
		if cmd.DeviceID == deviceID && cmd.MessageID == messageID {
			found = cmd
			commandID = key.(string)
			return false // 停止遍历
		}
		return true
	})

	if found == nil {
		logger.Warn("未找到对应的等待命令")
		return false
	}

	// 删除等待命令
	t.pendingCommands.Delete(commandID)

	// 取消超时
	found.Cancel()

	logger.Info("收到命令响应")

	// 通知响应
	if found.Callback != nil {
		go found.Callback(response, nil)
	}

	// 发送到响应通道
	select {
	case found.ResponseCh <- response:
	default:
		// 通道已满或关闭，忽略
	}

	return true
}

// WaitForResponse 等待响应
func (t *CommandResponseTracker) WaitForResponse(cmd *PendingCommand) (*dto.ChargeControlResponse, error) {
	select {
	case response := <-cmd.ResponseCh:
		return response, nil
	case <-cmd.Context.Done():
		// 清理等待命令
		t.pendingCommands.Delete(cmd.ID)
		return nil, fmt.Errorf("命令响应超时")
	}
}

// handleCommandTimeout 处理命令超时
func (t *CommandResponseTracker) handleCommandTimeout(cmd *PendingCommand) {
	<-cmd.Context.Done()

	// 检查是否因为超时还是正常响应
	if cmd.Context.Err() == context.DeadlineExceeded {
		logger.Warn("命令响应超时")

		// 调用超时回调
		if cmd.Callback != nil {
			go cmd.Callback(nil, fmt.Errorf("命令响应超时"))
		}

		// 清理等待命令
		t.pendingCommands.Delete(cmd.ID)
	}
}

// startCleanup 启动清理协程
func (t *CommandResponseTracker) startCleanup() {
	for {
		select {
		case <-t.cleanupTicker.C:
			t.cleanupExpiredCommands()
		case <-t.stopCleanup:
			return
		}
	}
}

// cleanupExpiredCommands 清理过期命令
func (t *CommandResponseTracker) cleanupExpiredCommands() {
	now := time.Now()
	expiredCommands := make([]string, 0)

	t.pendingCommands.Range(func(key, value interface{}) bool {
		cmd := value.(*PendingCommand)
		if now.Sub(cmd.CreatedAt) > cmd.Timeout {
			expiredCommands = append(expiredCommands, key.(string))
		}
		return true
	})

	// 清理过期命令
	for _, commandID := range expiredCommands {
		if cmdVal, exists := t.pendingCommands.LoadAndDelete(commandID); exists {
			cmd := cmdVal.(*PendingCommand)
			cmd.Cancel()
			logger.WithFields(logrus.Fields{
				"commandId": commandID,
				"deviceId":  cmd.DeviceID,
				"elapsed":   time.Since(cmd.CreatedAt).String(),
			}).Debug("清理过期命令")
		}
	}

	if len(expiredCommands) > 0 {
		logger.Info("清理过期命令完成")
	}
}

// GetPendingCommandsCount 获取等待中命令数量
func (t *CommandResponseTracker) GetPendingCommandsCount() int {
	count := 0
	t.pendingCommands.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Stop 停止跟踪器
func (t *CommandResponseTracker) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// 安全停止清理协程
	select {
	case <-t.stopCleanup:
		// 通道已经关闭
	default:
		close(t.stopCleanup)
	}
	t.cleanupTicker.Stop()

	// 取消所有等待中的命令
	t.pendingCommands.Range(func(key, value interface{}) bool {
		cmd := value.(*PendingCommand)
		cmd.Cancel()
		return true
	})

	logger.Info("命令响应跟踪器已停止")
}

// 全局命令响应跟踪器
var (
	globalCommandTracker *CommandResponseTracker
	trackerOnce          sync.Once
)

// GetGlobalCommandTracker 获取全局命令响应跟踪器
func GetGlobalCommandTracker() *CommandResponseTracker {
	trackerOnce.Do(func() {
		globalCommandTracker = NewCommandResponseTracker()
	})
	return globalCommandTracker
}
