// Package constants 定义了项目中使用的各种常量
package constants

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg/errors"
)

// DeviceConnectionState 统一的设备连接状态类型
// 替换原有的 ConnStatus 和 DeviceStatus，提供一致的状态管理
type DeviceConnectionState string

// ConnStatus 保持向后兼容，实际使用 DeviceConnectionState
type ConnStatus = DeviceConnectionState

// DeviceStatus 保持向后兼容，实际使用 DeviceConnectionState
type DeviceStatus = DeviceConnectionState

// 🔧 新增：连接属性键，用于在 Zinx 的 IConnection 中安全地存取属性
const (
	PropKeyConnStatus          = "connState"        // 连接状态 (建议使用 PropKeyConnectionState)
	PropKeyDeviceStatus        = "deviceStatus"     // 设备状态
	PropKeyDeviceId            = "deviceId"         // 设备ID
	PropKeyICCID               = "iccid"            // ICCID
	PropKeyPhysicalId          = "physicalId"       // 物理ID
	PropKeyConnectionState     = "connState"        // 连接状态
	PropKeyLastHeartbeat       = "lastHeartbeat"    // 最后心跳时间 (Unix timestamp)
	PropKeyLastHeartbeatStr    = "lastHeartbeatStr" // 最后心跳时间 (字符串格式)
	PropKeyReconnectCount      = "reconnectCount"   // 重连次数
	PropKeySessionID           = "sessionID"        // 会话ID
	PropKeyDeviceSession       = "deviceSession"    // 设备会话对象
	PropKeyDeviceSessionPrefix = "session:"         // 设备会话在Redis中的存储前缀
)

// 🔧 新增：函数类型定义，用于回调和依赖注入
type UpdateDeviceStatusFuncType func(deviceID string, status DeviceStatus) error

// 🔧 统一状态常量定义 - 使用 DeviceConnectionState 作为基础类型
const (
	// 基础连接状态
	StateConnected     DeviceConnectionState = "connected"      // TCP连接已建立
	StateICCIDReceived DeviceConnectionState = "iccid_received" // 已接收ICCID
	StateRegistered    DeviceConnectionState = "registered"     // 设备已注册
	StateOnline        DeviceConnectionState = "online"         // 设备在线（心跳正常）
	StateOffline       DeviceConnectionState = "offline"        // 设备离线
	StateDisconnected  DeviceConnectionState = "disconnected"   // 连接已断开
	StateError         DeviceConnectionState = "error"          // 连接错误状态
	StateUnknown       DeviceConnectionState = "unknown"        // 状态未知

	// 向后兼容的别名 - 保持现有代码正常工作
	ConnStatusConnected        = StateConnected
	ConnStatusAwaitingICCID    = StateICCIDReceived // 映射到新的状态
	ConnStatusICCIDReceived    = StateICCIDReceived
	ConnStatusActiveRegistered = StateRegistered
	ConnStatusOnline           = StateOnline
	ConnStatusClosed           = StateDisconnected
	ConnStatusInactive         = StateOffline
	ConnStatusActive           = StateOnline
	ConnStateAwaitingICCID     = StateICCIDReceived

	// 设备状态别名
	DeviceStatusOffline      = StateOffline
	DeviceStatusOnline       = StateOnline
	DeviceStatusReconnecting = StateError // 重连状态映射为错误状态
	DeviceStatusUnknown      = StateUnknown
)

// IsConsideredActive 检查一个连接状态是否被认为是“活跃”的（即已注册或在线）
func (cs ConnStatus) IsConsideredActive() bool {
	switch cs {
	case ConnStatusActiveRegistered, ConnStatusOnline:
		return true
	default:
		return false
	}
}

// IsActive 判断状态是否为活跃状态（可以进行业务操作）
func (s DeviceConnectionState) IsActive() bool {
	switch s {
	case StateRegistered, StateOnline:
		return true
	default:
		return false
	}
}

// IsConnected 判断是否有TCP连接
func (s DeviceConnectionState) IsConnected() bool {
	switch s {
	case StateConnected, StateICCIDReceived, StateRegistered, StateOnline:
		return true
	default:
		return false
	}
}

// CanReceiveCommands 判断是否可以接收命令
func (s DeviceConnectionState) CanReceiveCommands() bool {
	// 🔧 修复：设备注册后就应该能接收命令，不需要等到在线状态
	return s == StateRegistered || s == StateOnline
}

// String 返回状态的字符串表示
func (s DeviceConnectionState) String() string {
	return string(s)
}

// 🔧 状态转换规则定义
var StateTransitions = map[DeviceConnectionState][]DeviceConnectionState{
	StateConnected: {
		StateICCIDReceived, // 接收到ICCID
		StateDisconnected,  // 连接断开
		StateError,         // 连接错误
	},
	StateICCIDReceived: {
		StateRegistered,   // 设备注册成功
		StateDisconnected, // 连接断开
		StateError,        // 连接错误
	},
	StateRegistered: {
		StateOnline,       // 开始接收心跳
		StateDisconnected, // 连接断开
		StateError,        // 连接错误
	},
	StateOnline: {
		StateOffline,      // 心跳超时
		StateDisconnected, // 连接断开
		StateError,        // 连接错误
	},
	StateOffline: {
		StateOnline,       // 心跳恢复
		StateDisconnected, // 连接断开
		StateError,        // 连接错误
	},
}

// IsValidTransition 检查状态转换是否有效
func (s DeviceConnectionState) IsValidTransition(newState DeviceConnectionState) bool {
	if validStates, exists := StateTransitions[s]; exists {
		for _, validState := range validStates {
			if validState == newState {
				return true
			}
		}
	}
	return false
}

// DeviceError 设备相关错误
type DeviceError struct {
	Code     errors.ErrorCode
	Message  string
	DeviceID string
	Details  map[string]interface{}
}

func (e *DeviceError) Error() string {
	if e.DeviceID != "" {
		return fmt.Sprintf("[%d] 设备 %s: %s", e.Code, e.DeviceID, e.Message)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// NewDeviceError 创建设备错误
func NewDeviceError(code errors.ErrorCode, deviceID, message string) *DeviceError {
	return &DeviceError{
		Code:     code,
		Message:  message,
		DeviceID: deviceID,
		Details:  make(map[string]interface{}),
	}
}
