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

	// CommandRetryCount 命令重试次数上限(2次)
	CommandRetryCount = 2

	// CommandMaxAge 命令最大生命周期(60秒)
	// 无论重试次数，一个命令从创建到自动清除的最大时间
	CommandMaxAge = 60 * time.Second
)

// CommandEntry 命令条目
type CommandEntry struct {
	Connection   ziface.IConnection
	ConnID       uint64 // 保存连接ID，用于快速判断连接是否变化
	PhysicalID   uint32
	MessageID    uint16
	Command      uint8
	Data         []byte
	CreateTime   time.Time
	RetryCount   int
	LastSentTime time.Time
	Confirmed    bool // 是否已确认
}

// CommandManager 命令管理器
type CommandManager struct {
	commands         map[string]*CommandEntry // 命令映射表（主键）
	connCommands     map[uint64][]string      // 连接ID到命令键的映射
	physicalCommands map[uint32][]string      // 物理ID到命令键的映射
	lock             sync.RWMutex
	stopChan         chan struct{}
	isRunning        bool
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
			commands:         make(map[string]*CommandEntry),
			connCommands:     make(map[uint64][]string),
			physicalCommands: make(map[uint32][]string),
			stopChan:         make(chan struct{}),
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

// GenerateCommandKey 生成命令唯一标识
// 使用连接ID-物理ID-消息ID-命令 作为唯一键
func (cm *CommandManager) GenerateCommandKey(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8) string {
	return fmt.Sprintf("%d-%d-%d-%d", conn.GetConnID(), physicalID, messageID, command)
}

// RegisterCommand 注册命令
func (cm *CommandManager) RegisterCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) {
	if conn == nil {
		logger.Error("无法注册命令，连接为空")
		return
	}

	connID := conn.GetConnID()

	// 生成命令唯一标识
	cmdKey := cm.GenerateCommandKey(conn, physicalID, messageID, command)

	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 创建命令条目
	entry := &CommandEntry{
		Connection:   conn,
		ConnID:       connID,
		PhysicalID:   physicalID,
		MessageID:    messageID,
		Command:      command,
		Data:         data,
		CreateTime:   time.Now(),
		RetryCount:   0,
		LastSentTime: time.Now(),
		Confirmed:    false,
	}

	// 存储命令
	cm.commands[cmdKey] = entry

	// 更新连接ID到命令的映射
	cm.connCommands[connID] = append(cm.connCommands[connID], cmdKey)

	// 更新物理ID到命令的映射
	cm.physicalCommands[physicalID] = append(cm.physicalCommands[physicalID], cmdKey)

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"cmdKey":     cmdKey,
	}).Debug("注册新命令")
}

// ConfirmCommand 确认命令已完成
func (cm *CommandManager) ConfirmCommand(physicalID uint32, messageID uint16, command uint8) bool {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 查找所有关联到该物理ID的命令
	cmdKeys, exists := cm.physicalCommands[physicalID]
	if !exists {
		return false
	}

	confirmed := false

	// 检查每个命令是否匹配
	for _, cmdKey := range cmdKeys {
		cmd, exists := cm.commands[cmdKey]
		if !exists {
			continue
		}

		// 检查消息ID和命令是否匹配
		if cmd.MessageID == messageID && cmd.Command == command {
			// 标记为已确认
			cmd.Confirmed = true
			confirmed = true

			logger.WithFields(logrus.Fields{
				"physicalID": fmt.Sprintf("0x%08X", physicalID),
				"messageID":  messageID,
				"command":    fmt.Sprintf("0x%02X", command),
				"cmdKey":     cmdKey,
			}).Debug("确认命令已完成")
		}
	}

	// 清理已确认的命令
	cm.cleanupConfirmedCommands()

	return confirmed
}

// cleanupConfirmedCommands 清理已确认的命令
func (cm *CommandManager) cleanupConfirmedCommands() {
	// 已在调用方加锁，这里不需要再加锁

	var toDelete []string

	// 查找所有已确认的命令
	for cmdKey, cmd := range cm.commands {
		if cmd.Confirmed {
			toDelete = append(toDelete, cmdKey)
		}
	}

	// 删除已确认的命令
	for _, cmdKey := range toDelete {
		cm.deleteCommand(cmdKey)
	}
}

// ClearConnectionCommands 清理指定连接的所有命令
func (cm *CommandManager) ClearConnectionCommands(connID uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 获取连接关联的所有命令键
	cmdKeys, exists := cm.connCommands[connID]
	if !exists {
		return
	}

	// 删除所有关联的命令
	for _, cmdKey := range cmdKeys {
		cm.deleteCommand(cmdKey)
	}

	// 删除连接映射
	delete(cm.connCommands, connID)

	logger.WithFields(logrus.Fields{
		"connID":       connID,
		"commandCount": len(cmdKeys),
	}).Info("已清理连接的所有命令")
}

// deleteCommand 删除指定命令（内部方法，调用前需加锁）
func (cm *CommandManager) deleteCommand(cmdKey string) {
	cmd, exists := cm.commands[cmdKey]
	if !exists {
		return
	}

	// 从主映射表删除
	delete(cm.commands, cmdKey)

	// 从连接映射表删除
	connID := cmd.ConnID
	cmdKeys := cm.connCommands[connID]
	for i, key := range cmdKeys {
		if key == cmdKey {
			// 删除元素（保持顺序）
			if i < len(cmdKeys)-1 {
				copy(cmdKeys[i:], cmdKeys[i+1:])
			}
			cmdKeys = cmdKeys[:len(cmdKeys)-1]
			cm.connCommands[connID] = cmdKeys
			break
		}
	}

	// 从物理ID映射表删除
	physicalID := cmd.PhysicalID
	pCmdKeys := cm.physicalCommands[physicalID]
	for i, key := range pCmdKeys {
		if key == cmdKey {
			// 删除元素（保持顺序）
			if i < len(pCmdKeys)-1 {
				copy(pCmdKeys[i:], pCmdKeys[i+1:])
			}
			pCmdKeys = pCmdKeys[:len(pCmdKeys)-1]
			cm.physicalCommands[physicalID] = pCmdKeys
			break
		}
	}

	logger.WithFields(logrus.Fields{
		"cmdKey":     cmdKey,
		"connID":     connID,
		"physicalID": fmt.Sprintf("0x%08X", cmd.PhysicalID),
	}).Debug("已删除命令")
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
	var expiredCommands []string

	// 获取超时命令
	cm.lock.RLock()
	for key, cmd := range cm.commands {
		// 检查命令是否已确认
		if cmd.Confirmed {
			continue
		}

		// 检查命令是否超过最大生命周期
		if now.Sub(cmd.CreateTime) > CommandMaxAge {
			expiredCommands = append(expiredCommands, key)
			logger.WithFields(logrus.Fields{
				"cmdKey":     key,
				"physicalID": fmt.Sprintf("0x%08X", cmd.PhysicalID),
				"messageID":  cmd.MessageID,
				"command":    fmt.Sprintf("0x%02X", cmd.Command),
				"createTime": cmd.CreateTime.Format("15:04:05"),
				"age":        now.Sub(cmd.CreateTime).Seconds(),
			}).Info("命令超过最大生命周期，将被删除")
			continue
		}

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

	// 删除过期命令
	if len(expiredCommands) > 0 {
		cm.lock.Lock()
		for _, key := range expiredCommands {
			cm.deleteCommand(key)
		}
		cm.lock.Unlock()
	}

	// 处理超时命令
	for _, cmd := range timeoutCommands {
		cmdKey := cm.GenerateCommandKey(cmd.Connection, cmd.PhysicalID, cmd.MessageID, cmd.Command)

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

			cm.deleteCommand(cmdKey)
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

		// 检查连接是否有效
		if existingCmd.Connection == nil || existingCmd.Connection.GetConnID() != existingCmd.ConnID {
			logger.WithFields(logrus.Fields{
				"cmdKey":     cmdKey,
				"physicalID": fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
				"messageID":  existingCmd.MessageID,
				"command":    fmt.Sprintf("0x%02X", existingCmd.Command),
			}).Error("连接已变更，无法重发命令")

			// 删除无效连接的命令
			cm.lock.Lock()
			cm.deleteCommand(cmdKey)
			cm.lock.Unlock()
			continue
		}

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
				cm.deleteCommand(cmdKey)
				cm.lock.Unlock()
			}
		} else {
			logger.Error("未设置命令发送函数，无法重发命令")
		}
	}
}

// 命令发送函数类型定义
type SendCommandFuncType func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error

// 命令发送函数
var SendCommandFunc SendCommandFuncType

// SetSendCommandFunc 设置命令发送函数
func SetSendCommandFunc(fn SendCommandFuncType) {
	SendCommandFunc = fn
}
