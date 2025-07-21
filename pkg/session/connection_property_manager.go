package session

import (
	"sync"
)

// ConnectionPropertyManager 连接属性管理器
// 提供线程安全的键值对存储，替代散乱的连接属性管理
type ConnectionPropertyManager struct {
	properties sync.Map // 使用 sync.Map 提供线程安全的并发访问
}

// NewConnectionPropertyManager 创建新的连接属性管理器
func NewConnectionPropertyManager() *ConnectionPropertyManager {
	return &ConnectionPropertyManager{}
}

// SetProperty 设置属性
func (pm *ConnectionPropertyManager) SetProperty(key string, value interface{}) {
	pm.properties.Store(key, value)
}

// GetProperty 获取属性
// 返回值：value 属性值，exists 是否存在该属性
func (pm *ConnectionPropertyManager) GetProperty(key string) (interface{}, bool) {
	return pm.properties.Load(key)
}

// RemoveProperty 移除属性
func (pm *ConnectionPropertyManager) RemoveProperty(key string) {
	pm.properties.Delete(key)
}

// GetAllProperties 获取所有属性
// 返回所有属性的拷贝，避免并发问题
func (pm *ConnectionPropertyManager) GetAllProperties() map[string]interface{} {
	result := make(map[string]interface{})
	pm.properties.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			result[k] = value
		}
		return true
	})
	return result
}

// HasProperty 检查属性是否存在
func (pm *ConnectionPropertyManager) HasProperty(key string) bool {
	_, exists := pm.properties.Load(key)
	return exists
}

// Clear 清空所有属性
func (pm *ConnectionPropertyManager) Clear() {
	pm.properties.Range(func(key, _ interface{}) bool {
		pm.properties.Delete(key)
		return true
	})
}

// PropertyCount 获取属性数量
func (pm *ConnectionPropertyManager) PropertyCount() int {
	count := 0
	pm.properties.Range(func(_, value interface{}) bool {
		count++
		return true
	})
	return count
}
