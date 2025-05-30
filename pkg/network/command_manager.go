package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

const (
	// CommandTimeout 命令超时时间(15秒)
	CommandTimeout = 15 * time.Second

	// CommandRetryCount 命令重试次数(1次)
	CommandRetryCount = 1
)

// CommandEntry 命令条目
type CommandEntry struct {
	Connection   ziface.IConnection
	PhysicalID   uint32
	MessageID    uint16
	Command      uint8
	Data         []byte
	CreateTime   time.Time
	RetryCount   int
	LastSentTime time.Time
}

// CommandManager 命令管理器
type CommandManager struct {
	commands  map[string]*CommandEntry
	lock      sync.RWMutex
	stopChan  chan struct{}
	isRunning bool
}

// 确保CommandManager实现了ICommandManager接口
var _ ICommandManager = (*CommandManager)(nil)

// 创建全局命令管理器实例
var (
	globalCommandManager *CommandManager
	cmdMgrOnce           sync.Once
)

// GetCommandManager 获取全局命令管理器实例
func GetCommandManager() *CommandManager {
	cmdMgrOnce.Do(func() {
		globalCommandManager = &CommandManager{
			commands: make(map[string]*CommandEntry),
			stopChan: make(chan struct{}),
		}
	})
	return globalCommandManager
}

// Start 启动命令管理器
func (cm *CommandManager) Start() {
	cm.lock.Lock()
	if cm.isRunning {
		cm.lock.Unlock()
		return
	}
	cm.isRunning = true
	cm.lock.Unlock()

	logger.Info("命令管理器已启动，处理命令超时和重发")

	// 启动命令超时监控协程
	go cm.monitorCommands()
}

// Stop 停止命令管理器
func (cm *CommandManager) Stop() {
	cm.lock.Lock()
	if !cm.isRunning {
		cm.lock.Unlock()
		return
	}
	cm.isRunning = false
	close(cm.stopChan)
	cm.lock.Unlock()

	logger.Info("命令管理器已停止")
}

// RegisterCommand 注册命令
func (cm *CommandManager) RegisterCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) {
	// 生成命令唯一标识
	cmdKey := fmt.Sprintf("%d-%d-%d", physicalID, messageID, command)

	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 创建命令条目
	entry := &CommandEntry{
		Connection:   conn,
		PhysicalID:   physicalID,
		MessageID:    messageID,
		Command:      command,
		Data:         data,
		CreateTime:   time.Now(),
		RetryCount:   0,
		LastSentTime: time.Now(),
	}

	// 存储命令
	cm.commands[cmdKey] = entry

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"cmdKey":     cmdKey,
	}).Debug("注册新命令")
}

// ConfirmCommand 确认命令已完成
func (cm *CommandManager) ConfirmCommand(physicalID uint32, messageID uint16, command uint8) bool {
	// 生成命令唯一标识
	cmdKey := fmt.Sprintf("%d-%d-%d", physicalID, messageID, command)

	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 检查命令是否存在
	_, exists := cm.commands[cmdKey]
	if !exists {
		return false
	}

	// 删除命令
	delete(cm.commands, cmdKey)

	logger.WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"cmdKey":     cmdKey,
	}).Debug("确认命令已完成")

	return true
}

// monitorCommands 监控命令超时并处理重发
func (cm *CommandManager) monitorCommands() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopChan:
			return
		case <-ticker.C:
			cm.checkTimeoutCommands()
		}
	}
}

// checkTimeoutCommands 检查超时命令并处理
func (cm *CommandManager) checkTimeoutCommands() {
	now := time.Now()
	var timeoutCommands []*CommandEntry

	// 获取超时命令
	cm.lock.RLock()
	for key, cmd := range cm.commands {
		// 检查命令是否超时
		if now.Sub(cmd.LastSentTime) > CommandTimeout {
			timeoutCommands = append(timeoutCommands, cmd)
			logger.WithFields(logrus.Fields{
				"cmdKey":     key,
				"physicalID": fmt.Sprintf("0x%08X", cmd.PhysicalID),
				"messageID":  cmd.MessageID,
				"command":    fmt.Sprintf("0x%02X", cmd.Command),
				"retryCount": cmd.RetryCount,
				"timeSince":  now.Sub(cmd.LastSentTime).Seconds(),
			}).Info("发现超时命令")
		}
	}
	cm.lock.RUnlock()

	// 处理超时命令
	for _, cmd := range timeoutCommands {
		cmdKey := fmt.Sprintf("%d-%d-%d", cmd.PhysicalID, cmd.MessageID, cmd.Command)

		cm.lock.Lock()
		// 再次检查命令是否存在，可能已被其他协程处理
		existingCmd, exists := cm.commands[cmdKey]
		if !exists {
			cm.lock.Unlock()
			continue
		}

		// 如果重试次数已达上限，删除命令
		if existingCmd.RetryCount >= CommandRetryCount {
			logger.WithFields(logrus.Fields{
				"cmdKey":     cmdKey,
				"physicalID": fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
				"messageID":  existingCmd.MessageID,
				"command":    fmt.Sprintf("0x%02X", existingCmd.Command),
				"retryCount": existingCmd.RetryCount,
			}).Warn("命令超过重试次数上限，已放弃")

			delete(cm.commands, cmdKey)
			cm.lock.Unlock()
			continue
		}

		// 增加重试次数
		existingCmd.RetryCount++
		existingCmd.LastSentTime = now
		cm.lock.Unlock()

		// 重发命令
		logger.WithFields(logrus.Fields{
			"cmdKey":     cmdKey,
			"physicalID": fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
			"messageID":  existingCmd.MessageID,
			"command":    fmt.Sprintf("0x%02X", existingCmd.Command),
			"retryCount": existingCmd.RetryCount,
		}).Info("重发超时命令")

		// 构建响应数据包并发送 (这里需要外部提供发送方法)
		if SendCommandFunc != nil {
			err := SendCommandFunc(existingCmd.Connection, existingCmd.PhysicalID, existingCmd.MessageID, existingCmd.Command, existingCmd.Data)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"cmdKey":     cmdKey,
					"physicalID": fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
					"messageID":  existingCmd.MessageID,
					"command":    fmt.Sprintf("0x%02X", existingCmd.Command),
					"error":      err.Error(),
				}).Error("重发命令失败")

				// 删除失败的命令
				cm.lock.Lock()
				delete(cm.commands, cmdKey)
				cm.lock.Unlock()
			}
		} else {
			logger.Error("未设置命令发送函数，无法重发命令")
		}
	}
}

// 定义命令发送函数类型
type SendCommandFuncType func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error

// SendCommandFunc 命令发送函数，需要外部设置
var SendCommandFunc SendCommandFuncType

// SetSendCommandFunc 设置命令发送函数
func SetSendCommandFunc(fn SendCommandFuncType) {
	SendCommandFunc = fn
}
