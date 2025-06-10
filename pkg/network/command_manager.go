package network

import (
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
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

	// CommandBatchSize 命令批处理大小
	CommandBatchSize = 100
)

// CommandStatus 命令状态类型
type CommandStatus string

// 命令状态常量
const (
	CmdStatusPending   CommandStatus = "pending"   // 待处理
	CmdStatusSent      CommandStatus = "sent"      // 已发送
	CmdStatusRetrying  CommandStatus = "retrying"  // 重试中
	CmdStatusConfirmed CommandStatus = "confirmed" // 已确认
	CmdStatusFailed    CommandStatus = "failed"    // 失败
	CmdStatusExpired   CommandStatus = "expired"   // 过期
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
	Confirmed    bool          // 是否已确认
	Priority     int           // 命令优先级，值越小优先级越高
	Status       CommandStatus // 命令状态
	LastError    string        // 最后一次错误信息
}

// CommandManager 命令管理器
type CommandManager struct {
	// 命令映射
	commands map[string]*CommandEntry // map[cmdKey]*CommandEntry
	// 物理ID到命令的映射
	physicalCommands map[uint32][]string // map[physicalID][]cmdKey

	// 锁保护
	lock sync.Mutex

	// 批量处理命令配置
	batchProcessInterval time.Duration
	processingTicker     *time.Ticker
	stopChan             chan struct{}
	isRunning            bool
	maxRetry             int
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
			physicalCommands: make(map[uint32][]string),
			stopChan:         make(chan struct{}),
			maxRetry:         CommandRetryCount,
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
	return fmt.Sprintf("%d-0x%08X-%d-%d", conn.GetConnID(), physicalID, messageID, command)
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

	// 检查相同物理ID的相同命令是否已存在，如果存在则更新而不是添加新条目
	if cmdKeys, exists := cm.physicalCommands[physicalID]; exists {
		for _, key := range cmdKeys {
			if existingCmd, ok := cm.commands[key]; ok &&
				existingCmd.Command == command &&
				existingCmd.ConnID == connID {
				// 更新已存在的命令条目
				existingCmd.MessageID = messageID
				existingCmd.Data = data
				existingCmd.LastSentTime = time.Now()
				existingCmd.RetryCount = 0
				existingCmd.Confirmed = false
				existingCmd.Status = CmdStatusSent
				existingCmd.LastError = ""

				logger.WithFields(logrus.Fields{
					"connID":      connID,
					"physicalID":  fmt.Sprintf("0x%08X", physicalID),
					"messageID":   fmt.Sprintf("0x%04X (%d)", messageID, messageID),
					"command":     fmt.Sprintf("0x%02X", command),
					"commandDesc": GetCommandDescription(command),
					"cmdKey":      cmdKey,
					"dataLen":     len(data),
					"dataHex":     hex.EncodeToString(data),
					"priority":    existingCmd.Priority,
					"status":      existingCmd.Status,
				}).Debug("更新已存在的命令")

				return
			}
		}
	}

	// 根据命令类型设置优先级
	priority := getCommandPriority(command)

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
		Priority:     priority,
		Status:       CmdStatusSent,
		LastError:    "",
	}

	// 存储命令
	cm.commands[cmdKey] = entry

	// 更新物理ID到命令的映射
	cm.physicalCommands[physicalID] = append(cm.physicalCommands[physicalID], cmdKey)

	// 获取设备ICCID信息（如果有）
	var iccid string
	if iccidVal, err := conn.GetProperty(constants.PropKeyICCID); err == nil && iccidVal != nil {
		if val, ok := iccidVal.(string); ok {
			iccid = val
		}
	}

	// 获取远程地址信息
	remoteAddr := conn.RemoteAddr().String()

	logger.WithFields(logrus.Fields{
		"connID":      connID,
		"physicalID":  fmt.Sprintf("0x%08X", physicalID),
		"messageID":   fmt.Sprintf("0x%04X (%d)", messageID, messageID),
		"command":     fmt.Sprintf("0x%02X", command),
		"commandDesc": GetCommandDescription(command),
		"cmdKey":      cmdKey,
		"dataLen":     len(data),
		"dataHex":     hex.EncodeToString(data),
		"priority":    priority,
		"status":      entry.Status,
		"iccid":       iccid,
		"remoteAddr":  remoteAddr,
	}).Info("注册新命令")
}

// getCommandPriority 根据命令类型获取优先级
// 优先级值越小优先级越高，0为最高优先级
func getCommandPriority(command uint8) int {
	switch command {
	case 0x20: // 设备注册 - 最高优先级
		return 0
	case 0x25: // 充电控制 - 高优先级
		return 1
	case 0x21: // 刷卡 - 高优先级
		return 1
	case 0x23: // 结算 - 中高优先级
		return 2
	case 0x22: // 参数设置 - 中等优先级
		return 3
	case 0x26: // 获取服务器时间 - 中等优先级
		return 3
	case 0x01: // 心跳 - 低优先级
		return 5
	case 0x24: // 功率心跳 - 低优先级
		return 5
	default: // 未知命令 - 中等优先级
		return 3
	}
}

// ConfirmCommand 确认命令已完成
func (cm *CommandManager) ConfirmCommand(physicalID uint32, messageID uint16, command uint8) bool {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 查找所有关联到该物理ID的命令
	cmdKeys, exists := cm.physicalCommands[physicalID]
	if !exists {
		logger.WithFields(logrus.Fields{
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X (%d)", messageID, messageID),
			"command":    fmt.Sprintf("0x%02X", command),
		}).Debug("确认命令失败：未找到该物理ID的命令")
		return false
	}

	confirmed := false
	exactMatch := false

	// 检查每个命令是否匹配
	for _, cmdKey := range cmdKeys {
		cmd, exists := cm.commands[cmdKey]
		if !exists {
			continue
		}

		// 优先进行完全匹配（物理ID + messageID + command）
		if cmd.Command == command && cmd.MessageID == messageID {
			// 标记为已确认并更新状态
			cmd.Confirmed = true
			cmd.Status = CmdStatusConfirmed

			confirmed = true
			exactMatch = true

			logger.WithFields(logrus.Fields{
				"physicalID":       fmt.Sprintf("0x%08X", physicalID),
				"messageID":        fmt.Sprintf("0x%04X (%d)", messageID, messageID),
				"command":          fmt.Sprintf("0x%02X", command),
				"cmdKey":           cmdKey,
				"matchType":        "完全匹配",
				"originalMsgID":    fmt.Sprintf("0x%04X (%d)", cmd.MessageID, cmd.MessageID),
				"timeSinceCreated": time.Since(cmd.CreateTime).Seconds(),
				"retryCount":       cmd.RetryCount,
				"status":           cmd.Status,
				"dataHex":          hex.EncodeToString(cmd.Data),
			}).Info("确认命令已完成 - 完全匹配")

			// 已找到完全匹配，不再继续查找宽松匹配
			break
		}
	}

	// 如果没有找到完全匹配，尝试宽松匹配（兼容旧版本）
	if !exactMatch {
		for _, cmdKey := range cmdKeys {
			cmd, exists := cm.commands[cmdKey]
			if !exists {
				continue
			}

			// 宽松匹配（只匹配物理ID和command）
			if cmd.Command == command && !cmd.Confirmed {
				// 标记为已确认并更新状态
				cmd.Confirmed = true
				cmd.Status = CmdStatusConfirmed
				confirmed = true

				logger.WithFields(logrus.Fields{
					"physicalID":       fmt.Sprintf("0x%08X", physicalID),
					"messageID":        fmt.Sprintf("0x%04X (%d)", messageID, messageID),
					"command":          fmt.Sprintf("0x%02X", command),
					"cmdKey":           cmdKey,
					"matchType":        "宽松匹配",
					"originalMsgID":    fmt.Sprintf("0x%04X (%d)", cmd.MessageID, cmd.MessageID),
					"timeSinceCreated": time.Since(cmd.CreateTime).Seconds(),
					"retryCount":       cmd.RetryCount,
					"status":           cmd.Status,
					"dataHex":          hex.EncodeToString(cmd.Data),
					"warning":          "消息ID不匹配，但仍然确认命令 - 考虑升级为严格匹配",
				}).Warn("确认命令已完成 - 宽松匹配（兼容模式）")
			}
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

	// 找到该连接的所有命令
	var cmdKeysToDelete []string
	for key, cmd := range cm.commands {
		if cmd.ConnID == connID {
			cmdKeysToDelete = append(cmdKeysToDelete, key)
		}
	}

	// 删除这些命令
	for _, cmdKey := range cmdKeysToDelete {
		cm.deleteCommand(cmdKey)
	}

	logger.WithFields(logrus.Fields{
		"connID":       connID,
		"commandCount": len(cmdKeysToDelete),
	}).Info("已清理连接的所有命令")
}

// ClearPhysicalIDCommands 清理指定物理ID的所有命令
// 当设备重新连接或更换连接时使用
func (cm *CommandManager) ClearPhysicalIDCommands(physicalID uint32) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// 获取物理ID关联的所有命令键
	cmdKeys, exists := cm.physicalCommands[physicalID]
	if !exists {
		logger.WithField("physicalID", fmt.Sprintf("0x%08X", physicalID)).
			Debug("未找到物理ID关联的命令")
		return
	}

	// 删除所有关联的命令
	for _, cmdKey := range cmdKeys {
		cm.deleteCommand(cmdKey)
	}

	// 删除物理ID映射
	delete(cm.physicalCommands, physicalID)

	logger.WithFields(logrus.Fields{
		"physicalID":   fmt.Sprintf("0x%08X", physicalID),
		"commandCount": len(cmdKeys),
	}).Info("已清理物理ID的所有命令")
}

// deleteCommand 删除指定命令（内部方法，调用前需加锁）
func (cm *CommandManager) deleteCommand(cmdKey string) {
	cmd, exists := cm.commands[cmdKey]
	if !exists {
		return
	}

	// 从主映射表删除
	delete(cm.commands, cmdKey)

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
		"connID":     cmd.ConnID,
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
	var expiredCommandKeys []string
	var expiredCommands []*CommandEntry // 保存过期命令的引用

	// 批量收集超时和过期命令，减少锁持有时间
	cm.lock.Lock()
	for key, cmd := range cm.commands {
		// 检查命令是否已确认
		if cmd.Confirmed {
			continue
		}

		// 检查命令是否超过最大生命周期
		if now.Sub(cmd.CreateTime) > CommandMaxAge {
			expiredCommandKeys = append(expiredCommandKeys, key)

			// 更新命令状态为过期
			cmd.Status = CmdStatusExpired
			cmd.LastError = fmt.Sprintf("命令超过最大生命周期 (%.2f秒)", now.Sub(cmd.CreateTime).Seconds())

			// 保存命令引用用于日志记录
			cmdCopy := *cmd
			expiredCommands = append(expiredCommands, &cmdCopy)

			logger.WithFields(logrus.Fields{
				"cmdKey":      key,
				"physicalID":  fmt.Sprintf("0x%08X", cmd.PhysicalID),
				"messageID":   fmt.Sprintf("0x%04X (%d)", cmd.MessageID, cmd.MessageID),
				"command":     fmt.Sprintf("0x%02X", cmd.Command),
				"commandDesc": GetCommandDescription(cmd.Command),
				"createTime":  cmd.CreateTime.Format("15:04:05.000"),
				"age":         now.Sub(cmd.CreateTime).Seconds(),
				"status":      cmd.Status,
				"lastError":   cmd.LastError,
			}).Info("命令超过最大生命周期，将被删除")
			continue
		}

		// 检查命令是否超时
		if now.Sub(cmd.LastSentTime) > CommandTimeout {
			// 创建副本，避免后续处理时出现并发修改问题
			cmdCopy := *cmd
			timeoutCommands = append(timeoutCommands, &cmdCopy)
		}
	}
	cm.lock.Unlock()

	// 批量删除过期命令
	if len(expiredCommandKeys) > 0 {
		cm.lock.Lock()
		for _, key := range expiredCommandKeys {
			cm.deleteCommand(key)
		}
		cm.lock.Unlock()

		// 记录详细的过期命令信息
		for _, cmd := range expiredCommands {
			logger.WithFields(logrus.Fields{
				"physicalID":  fmt.Sprintf("0x%08X", cmd.PhysicalID),
				"messageID":   fmt.Sprintf("0x%04X (%d)", cmd.MessageID, cmd.MessageID),
				"command":     fmt.Sprintf("0x%02X", cmd.Command),
				"commandDesc": GetCommandDescription(cmd.Command),
				"connID":      cmd.ConnID,
				"createTime":  cmd.CreateTime.Format("15:04:05.000"),
				"age":         now.Sub(cmd.CreateTime).Seconds(),
				"retryCount":  cmd.RetryCount,
				"status":      cmd.Status,
				"lastError":   cmd.LastError,
				"dataHex":     hex.EncodeToString(cmd.Data),
			}).Debug("已删除过期命令详情")
		}

		logger.WithFields(logrus.Fields{
			"count":      len(expiredCommandKeys),
			"expireTime": CommandMaxAge.Seconds(),
		}).Info("已批量清理过期命令")
	}

	// 按批次处理超时命令，减少锁争用
	if len(timeoutCommands) > 0 {
		// 按优先级和物理ID排序，确保重要命令优先处理
		sort.Slice(timeoutCommands, func(i, j int) bool {
			// 首先按优先级排序（值越小优先级越高）
			if timeoutCommands[i].Priority != timeoutCommands[j].Priority {
				return timeoutCommands[i].Priority < timeoutCommands[j].Priority
			}
			// 其次按物理ID排序，保证同一设备的命令连续处理
			return timeoutCommands[i].PhysicalID < timeoutCommands[j].PhysicalID
		})

		// 按批次处理，每批最多处理CommandBatchSize个命令
		for i := 0; i < len(timeoutCommands); i += CommandBatchSize {
			end := i + CommandBatchSize
			if end > len(timeoutCommands) {
				end = len(timeoutCommands)
			}
			batch := timeoutCommands[i:end]

			// 处理当前批次
			cm.processBatchTimeoutCommands(batch)

			// 批次处理完后短暂休眠，避免网络拥塞
			if end < len(timeoutCommands) {
				time.Sleep(50 * time.Millisecond)
			}
		}

		logger.WithFields(logrus.Fields{
			"count":       len(timeoutCommands),
			"timeoutTime": CommandTimeout.Seconds(),
		}).Info("已批量处理超时命令")
	}
}

// processBatchTimeoutCommands 批量处理超时命令
func (cm *CommandManager) processBatchTimeoutCommands(commands []*CommandEntry) {
	for _, cmd := range commands {
		cmdKey := cm.GenerateCommandKey(cmd.Connection, cmd.PhysicalID, cmd.MessageID, cmd.Command)

		// 先检查命令是否仍然需要重试
		cm.lock.Lock()
		existingCmd, exists := cm.commands[cmdKey]
		if !exists || existingCmd.Confirmed {
			cm.lock.Unlock()
			continue
		}

		// 日志记录超时情况
		logger.WithFields(logrus.Fields{
			"cmdKey":      cmdKey,
			"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
			"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
			"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
			"commandDesc": GetCommandDescription(existingCmd.Command),
			"retryCount":  existingCmd.RetryCount,
			"timeSince":   time.Since(existingCmd.LastSentTime).Seconds(),
			"createTime":  existingCmd.CreateTime.Format("15:04:05.000"),
			"connID":      existingCmd.ConnID,
			"dataHex":     hex.EncodeToString(existingCmd.Data),
			"status":      existingCmd.Status,
		}).Info("发现超时命令")

		// 如果重试次数已达上限，删除命令
		if existingCmd.RetryCount >= cm.maxRetry {
			// 更新状态为失败
			existingCmd.Status = CmdStatusFailed
			existingCmd.LastError = fmt.Sprintf("重试次数已达上限 (%d/%d)", existingCmd.RetryCount, cm.maxRetry)

			logger.WithFields(logrus.Fields{
				"cmdKey":      cmdKey,
				"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
				"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
				"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
				"commandDesc": GetCommandDescription(existingCmd.Command),
				"retryCount":  existingCmd.RetryCount,
				"maxRetry":    cm.maxRetry,
				"age":         time.Since(existingCmd.CreateTime).Seconds(),
				"status":      existingCmd.Status,
				"lastError":   existingCmd.LastError,
			}).Warn("命令重试次数已达上限，放弃重试")
			delete(cm.commands, cmdKey)
			cm.lock.Unlock()
			continue
		}

		// 检查连接是否仍然有效
		if !isConnectionActive(existingCmd.Connection) {
			// 更新状态为失败
			existingCmd.Status = CmdStatusFailed
			existingCmd.LastError = "连接已关闭"

			logger.WithFields(logrus.Fields{
				"cmdKey":      cmdKey,
				"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
				"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
				"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
				"commandDesc": GetCommandDescription(existingCmd.Command),
				"connID":      existingCmd.Connection.GetConnID(),
				"reason":      existingCmd.LastError,
				"status":      existingCmd.Status,
			}).Warn("命令重试失败，放弃重试")
			delete(cm.commands, cmdKey)
			cm.lock.Unlock()
			continue
		}

		// 增加重试次数并更新状态和最后发送时间
		existingCmd.RetryCount++
		existingCmd.Status = CmdStatusRetrying
		lastSentTime := existingCmd.LastSentTime // 保存上次发送时间
		existingCmd.LastSentTime = time.Now()

		// 为了避免在发送过程中锁定，先解锁
		cm.lock.Unlock()

		// 记录重发日志
		logger.WithFields(logrus.Fields{
			"cmdKey":      cmdKey,
			"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
			"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
			"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
			"commandDesc": GetCommandDescription(existingCmd.Command),
			"retryCount":  existingCmd.RetryCount,
			"timeSince":   time.Since(lastSentTime).Seconds(),
			"connID":      existingCmd.ConnID,
			"dataHex":     hex.EncodeToString(existingCmd.Data),
			"status":      existingCmd.Status,
		}).Info("重发超时命令")

		// 重发命令 - 确保使用原始的messageID
		if SendCommandFunc != nil {
			// 记录发送前的时间
			sendStartTime := time.Now()

			// 发送命令，使用原始参数
			err := SendCommandFunc(
				existingCmd.Connection,
				existingCmd.PhysicalID,
				existingCmd.MessageID, // 确保使用原始messageID
				existingCmd.Command,
				existingCmd.Data)

			// 计算发送耗时
			sendTime := time.Since(sendStartTime).Milliseconds()

			// 更新命令状态
			cm.lock.Lock()
			if cmd, exists := cm.commands[cmdKey]; exists {
				if err != nil {
					cmd.LastError = err.Error()
					logger.WithFields(logrus.Fields{
						"cmdKey":      cmdKey,
						"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
						"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
						"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
						"commandDesc": GetCommandDescription(existingCmd.Command),
						"retryCount":  existingCmd.RetryCount,
						"error":       err.Error(),
						"sendTime":    sendTime,
						"status":      cmd.Status,
					}).Error("重发超时命令失败")
				} else {
					cmd.Status = CmdStatusSent
					logger.WithFields(logrus.Fields{
						"cmdKey":      cmdKey,
						"physicalID":  fmt.Sprintf("0x%08X", existingCmd.PhysicalID),
						"messageID":   fmt.Sprintf("0x%04X (%d)", existingCmd.MessageID, existingCmd.MessageID),
						"command":     fmt.Sprintf("0x%02X", existingCmd.Command),
						"commandDesc": GetCommandDescription(existingCmd.Command),
						"retryCount":  existingCmd.RetryCount,
						"sendTime":    sendTime,
						"status":      cmd.Status,
					}).Debug("重发超时命令成功")
				}
			}
			cm.lock.Unlock()
		} else {
			logger.Error("未设置命令发送函数，无法重发命令")
		}
	}
}

// isConnectionActive 检查连接是否仍然活跃
func isConnectionActive(conn ziface.IConnection) bool {
	// 检查连接是否为nil
	if conn == nil || conn.GetTCPConnection() == nil {
		return false
	}

	// 检查连接状态
	if val, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && val != nil {
		status := val.(string)
		return status != constants.ConnStatusClosed && status != constants.ConnStatusInactive
	}

	// 无法确定状态时保守处理，认为连接有效
	return true
}

// 命令发送函数类型定义
type SendCommandFuncType func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error

// 命令发送函数
var SendCommandFunc SendCommandFuncType

// SetSendCommandFunc 设置命令发送函数
func SetSendCommandFunc(fn SendCommandFuncType) {
	SendCommandFunc = fn
}

// GetCommandDescription 获取命令描述
func GetCommandDescription(command uint8) string {
	// 根据命令码返回对应的描述信息
	switch command {
	case 0x01:
		return "心跳"
	case 0x12:
		return "主机获取服务器时间"
	case 0x20:
		return "设备注册"
	case 0x21:
		return "刷卡"
	case 0x22:
		return "参数设置"
	case 0x23:
		return "结算"
	case 0x24:
		return "功率心跳"
	case 0x25:
		return "充电控制"
	case 0x26:
		return "获取服务器时间"
	default:
		return fmt.Sprintf("未知命令(0x%02X)", command)
	}
}
