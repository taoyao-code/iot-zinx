package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ChargingState 充电状态枚举 - 修复CVE-Critical-002
type ChargingState int

const (
	StateIdle          ChargingState = iota // 空闲状态
	StatePlugged                            // 已插枪，等待充电
	StateCharging                           // 正在充电
	StateFloatCharging                      // 浮充状态
	StateCompleted                          // 充电完成
	StateFault                              // 故障状态
	StateEmergencyStop                      // 紧急停止
)

// String 返回充电状态的字符串表示
func (s ChargingState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StatePlugged:
		return "plugged"
	case StateCharging:
		return "charging"
	case StateFloatCharging:
		return "float_charging"
	case StateCompleted:
		return "completed"
	case StateFault:
		return "fault"
	case StateEmergencyStop:
		return "emergency_stop"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// StateChangeReason 状态变更原因
type StateChangeReason string

const (
	ReasonUserRequest    StateChangeReason = "user_request"
	ReasonDeviceResponse StateChangeReason = "device_response"
	ReasonHeartbeat      StateChangeReason = "heartbeat"
	ReasonTimeout        StateChangeReason = "timeout"
	ReasonFault          StateChangeReason = "fault"
	ReasonEmergency      StateChangeReason = "emergency"
	ReasonPowerAbnormal  StateChangeReason = "power_abnormal"
	ReasonSettlement     StateChangeReason = "settlement"
)

// StateChange 状态变更事件
type StateChange struct {
	DeviceID    string                 `json:"device_id"`
	Port        int                    `json:"port"`
	FromState   ChargingState          `json:"from_state"`
	ToState     ChargingState          `json:"to_state"`
	Reason      StateChangeReason      `json:"reason"`
	Timestamp   time.Time              `json:"timestamp"`
	OrderNo     string                 `json:"orderNo,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	ErrorDetail string                 `json:"error_detail,omitempty"`
}

// ChargingStateMachine 充电状态机 - 修复CVE-Critical-002
type ChargingStateMachine struct {
	currentState ChargingState
	deviceID     string
	port         int
	orderNo      string
	transitions  map[ChargingState][]ChargingState
	stateChanges chan StateChange
	mutex        sync.RWMutex
	lastUpdate   time.Time
	stateHistory []StateChange // 状态变更历史
}

// NewChargingStateMachine 创建新的充电状态机
func NewChargingStateMachine(deviceID string, port int) *ChargingStateMachine {
	csm := &ChargingStateMachine{
		currentState: StateIdle,
		deviceID:     deviceID,
		port:         port,
		stateChanges: make(chan StateChange, 100), // 缓冲区，防止阻塞
		lastUpdate:   time.Now(),
		stateHistory: make([]StateChange, 0, 50), // 保留最近50个状态变更
		transitions: map[ChargingState][]ChargingState{
			// 空闲状态可以转换到：已插枪、故障
			StateIdle: {StatePlugged, StateFault},

			// 已插枪可以转换到：正在充电、空闲、故障
			StatePlugged: {StateCharging, StateIdle, StateFault},

			// 正在充电可以转换到：浮充、完成、故障、紧急停止、空闲（拔枪）
			StateCharging: {StateFloatCharging, StateCompleted, StateFault, StateEmergencyStop, StateIdle},

			// 浮充可以转换到：完成、故障、紧急停止
			StateFloatCharging: {StateCompleted, StateFault, StateEmergencyStop},

			// 完成可以转换到：空闲、故障
			StateCompleted: {StateIdle, StateFault},

			// 故障可以转换到：空闲（故障修复后）
			StateFault: {StateIdle},

			// 紧急停止可以转换到：空闲、故障
			StateEmergencyStop: {StateIdle, StateFault},
		},
	}

	return csm
}

// GetCurrentState 获取当前状态
func (csm *ChargingStateMachine) GetCurrentState() ChargingState {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.currentState
}

// GetOrderNo 获取当前订单号
func (csm *ChargingStateMachine) GetOrderNo() string {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.orderNo
}

// SetOrderNo 设置订单号
func (csm *ChargingStateMachine) SetOrderNo(orderNo string) {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()
	csm.orderNo = orderNo
}

// TransitionTo 状态转换 - 带验证和事件通知
func (csm *ChargingStateMachine) TransitionTo(newState ChargingState, reason StateChangeReason, data map[string]interface{}) error {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	oldState := csm.currentState

	// 相同状态不需要转换
	if oldState == newState {
		return nil
	}

	// 验证状态转换的合法性
	allowedTransitions, exists := csm.transitions[oldState]
	if !exists {
		return fmt.Errorf("未定义状态 %v 的转换规则", oldState)
	}

	allowed := false
	for _, allowedState := range allowedTransitions {
		if allowedState == newState {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("不允许从 %v 转换到 %v", oldState, newState)
	}

	// 执行状态转换
	csm.currentState = newState
	csm.lastUpdate = time.Now()

	// 创建状态变更事件
	change := StateChange{
		DeviceID:  csm.deviceID,
		Port:      csm.port,
		FromState: oldState,
		ToState:   newState,
		Reason:    reason,
		Timestamp: csm.lastUpdate,
		OrderNo:   csm.orderNo,
		Data:      data,
	}

	// 记录到历史
	csm.addToHistoryUnsafe(change)

	// 记录日志
	logger.WithFields(logrus.Fields{
		"deviceID":  csm.deviceID,
		"port":      csm.port,
		"orderNo":   csm.orderNo,
		"fromState": oldState.String(),
		"toState":   newState.String(),
		"reason":    string(reason),
		"data":      data,
	}).Info("🔄 充电状态机状态转换")

	// 异步发送状态变更事件
	go func() {
		select {
		case csm.stateChanges <- change:
		default:
			// 队列满时记录警告
			logger.WithFields(logrus.Fields{
				"deviceID": csm.deviceID,
				"port":     csm.port,
				"change":   change,
			}).Warn("状态变更队列已满，丢弃事件")
		}
	}()

	return nil
}

// ProcessProtocolStatus 处理协议状态码 - 从心跳包解析状态
func (csm *ChargingStateMachine) ProcessProtocolStatus(protocolStatus uint8, reason StateChangeReason, data map[string]interface{}) error {
	var targetState ChargingState

	// 根据协议解析充电状态
	switch protocolStatus {
	case 0:
		targetState = StateIdle // 空闲
	case 1:
		targetState = StateCharging // 充电中
	case 2:
		targetState = StatePlugged // 已扫码，等待插入充电器
	case 3:
		targetState = StateCompleted // 有充电器但未充电（已充满）
	case 5:
		targetState = StateFloatCharging // 浮充
	default:
		// 未知状态，记录为故障
		targetState = StateFault
		if data == nil {
			data = make(map[string]interface{})
		}
		data["unknown_protocol_status"] = protocolStatus
	}

	return csm.TransitionTo(targetState, reason, data)
}

// HandleEmergencyStop 处理紧急停止
func (csm *ChargingStateMachine) HandleEmergencyStop(reason string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["emergency_reason"] = reason

	return csm.TransitionTo(StateEmergencyStop, ReasonEmergency, data)
}

// HandleFault 处理故障状态
func (csm *ChargingStateMachine) HandleFault(faultCode string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["fault_code"] = faultCode

	return csm.TransitionTo(StateFault, ReasonFault, data)
}

// GetStateHistory 获取状态变更历史
func (csm *ChargingStateMachine) GetStateHistory() []StateChange {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	// 返回历史副本
	history := make([]StateChange, len(csm.stateHistory))
	copy(history, csm.stateHistory)
	return history
}

// addToHistoryUnsafe 添加到历史记录（不加锁版本）
func (csm *ChargingStateMachine) addToHistoryUnsafe(change StateChange) {
	// 添加到历史
	csm.stateHistory = append(csm.stateHistory, change)

	// 保持历史记录数量限制
	if len(csm.stateHistory) > 50 {
		// 移除最老的记录
		csm.stateHistory = csm.stateHistory[1:]
	}
}

// GetLastUpdate 获取最后更新时间
func (csm *ChargingStateMachine) GetLastUpdate() time.Time {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.lastUpdate
}

// IsCharging 判断是否正在充电
func (csm *ChargingStateMachine) IsCharging() bool {
	state := csm.GetCurrentState()
	return state == StateCharging || state == StateFloatCharging
}

// CanStartCharging 判断是否可以开始充电
func (csm *ChargingStateMachine) CanStartCharging() bool {
	state := csm.GetCurrentState()
	return state == StateIdle || state == StatePlugged
}

// CanStopCharging 判断是否可以停止充电
func (csm *ChargingStateMachine) CanStopCharging() bool {
	state := csm.GetCurrentState()
	return state == StateCharging || state == StateFloatCharging
}

// GetStateChanges 获取状态变更通道（用于监听状态变更）
func (csm *ChargingStateMachine) GetStateChanges() <-chan StateChange {
	return csm.stateChanges
}

// Close 关闭状态机
func (csm *ChargingStateMachine) Close() {
	if csm.stateChanges != nil {
		close(csm.stateChanges)
	}

	logger.WithFields(logrus.Fields{
		"deviceID":   csm.deviceID,
		"port":       csm.port,
		"finalState": csm.GetCurrentState().String(),
		"orderNo":    csm.orderNo,
	}).Info("🔚 充电状态机已关闭")
}
