package constants

import (
	"fmt"
	"sync"
)

// CommandInfo 命令信息结构体
type CommandInfo struct {
	ID          uint8  // 命令ID
	Name        string // 命令名称
	Description string // 命令描述
	Category    string // 命令分类（如：心跳、注册、充电控制等）
	Priority    int    // 命令优先级（0为最高优先级）
}

// CommandRegistry 命令注册表
type CommandRegistry struct {
	commands map[uint8]*CommandInfo // 命令ID到命令信息的映射
	mutex    sync.RWMutex           // 读写锁保护并发访问
}

// NewCommandRegistry 创建新的命令注册表
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[uint8]*CommandInfo),
	}
}

// Register 注册命令信息
func (r *CommandRegistry) Register(info *CommandInfo) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.commands[info.ID] = info
}

// RegisterBatch 批量注册命令信息
func (r *CommandRegistry) RegisterBatch(infos []*CommandInfo) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, info := range infos {
		r.commands[info.ID] = info
	}
}

// GetCommandInfo 获取命令完整信息
func (r *CommandRegistry) GetCommandInfo(commandID uint8) (*CommandInfo, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	info, exists := r.commands[commandID]
	return info, exists
}

// GetCommandName 获取命令名称
func (r *CommandRegistry) GetCommandName(commandID uint8) string {
	if info, exists := r.GetCommandInfo(commandID); exists {
		return info.Name
	}
	return fmt.Sprintf("未知命令(0x%02X)", commandID)
}

// GetCommandDescription 获取命令描述
func (r *CommandRegistry) GetCommandDescription(commandID uint8) string {
	if info, exists := r.GetCommandInfo(commandID); exists {
		return info.Description
	}
	return fmt.Sprintf("未知命令(0x%02X)", commandID)
}

// GetCommandPriority 获取命令优先级
func (r *CommandRegistry) GetCommandPriority(commandID uint8) int {
	if info, exists := r.GetCommandInfo(commandID); exists {
		return info.Priority
	}
	return 3 // 默认中等优先级
}

// GetAllCommands 获取所有已注册的命令
func (r *CommandRegistry) GetAllCommands() map[uint8]*CommandInfo {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// 创建副本避免外部修改
	result := make(map[uint8]*CommandInfo)
	for id, info := range r.commands {
		// 创建CommandInfo的副本
		infoCopy := *info
		result[id] = &infoCopy
	}
	return result
}

// GetCommandsByCategory 根据分类获取命令列表
func (r *CommandRegistry) GetCommandsByCategory(category string) []*CommandInfo {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var result []*CommandInfo
	for _, info := range r.commands {
		if info.Category == category {
			// 创建副本
			infoCopy := *info
			result = append(result, &infoCopy)
		}
	}
	return result
}

// IsRegistered 检查命令是否已注册
func (r *CommandRegistry) IsRegistered(commandID uint8) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	_, exists := r.commands[commandID]
	return exists
}

// Count 获取已注册命令的数量
func (r *CommandRegistry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.commands)
}

// 全局命令注册表实例
var (
	globalRegistry *CommandRegistry
	registryOnce   sync.Once
)

// GetGlobalCommandRegistry 获取全局命令注册表实例
func GetGlobalCommandRegistry() *CommandRegistry {
	registryOnce.Do(func() {
		globalRegistry = NewCommandRegistry()
		// 初始化默认命令
		initDefaultCommands()
	})
	return globalRegistry
}

// 便捷函数，直接使用全局注册表
func GetCommandName(commandID uint8) string {
	return GetGlobalCommandRegistry().GetCommandName(commandID)
}

func GetCommandDescription(commandID uint8) string {
	return GetGlobalCommandRegistry().GetCommandDescription(commandID)
}

func GetCommandPriority(commandID uint8) int {
	return GetGlobalCommandRegistry().GetCommandPriority(commandID)
}

func GetCommandInfo(commandID uint8) (*CommandInfo, bool) {
	return GetGlobalCommandRegistry().GetCommandInfo(commandID)
}
