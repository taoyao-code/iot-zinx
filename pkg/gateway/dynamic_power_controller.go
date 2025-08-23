package gateway

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DynamicPowerController 智能降功率控制器
type DynamicPowerController struct {
	cfg     config.SmartChargingConfig
	mu      sync.RWMutex
	entries map[string]*dpcEntry // key: deviceID|port
}

type dpcEntry struct {
	deviceID   string
	port1Based int
	orderNo    string

	firstChargingAt time.Time
	lastAdjustAt    time.Time
	lastOverloadW   int

	// 对账相关
	lastObservedW int
	lastTargetW   int
	lastTargetAt  time.Time
	resentOnce    bool
}

var (
	globalDPC     *DynamicPowerController
	globalDPCOnce sync.Once
)

// InitDynamicPowerController 初始化控制器
func InitDynamicPowerController() {
	globalDPCOnce.Do(func() {
		globalDPC = &DynamicPowerController{
			cfg:     config.GetConfig().SmartCharging,
			entries: make(map[string]*dpcEntry),
		}
		logger.Info("智能降功率控制器已初始化")
	})
}

// GetDynamicPowerController 获取全局控制器
func GetDynamicPowerController() *DynamicPowerController {
	if globalDPC == nil {
		InitDynamicPowerController()
	}
	return globalDPC
}

func makeKey(deviceID string, port1Based int) string {
	return fmt.Sprintf("%s|%d", deviceID, port1Based)
}

// OnPowerHeartbeat 由 06/26 心跳回调
// port1Based: 业务侧端口(从1开始)。realtimePowerW: 当前功率(瓦)。isCharging: 是否充电中。
func (d *DynamicPowerController) OnPowerHeartbeat(deviceID string, port1Based int, orderNo string, realtimePowerW int, isCharging bool, observedAt time.Time) {
	if d == nil || !d.cfg.Enabled {
		return
	}
	if port1Based <= 0 || deviceID == "" || orderNo == "" || !isCharging {
		return
	}

	// 采样
	if d.cfg.SampleRate > 1 {
		if (observedAt.Unix() % int64(d.cfg.SampleRate)) != 0 {
			return
		}
	}

	key := makeKey(deviceID, port1Based)

	d.mu.Lock()
	entry, ok := d.entries[key]
	if !ok || entry == nil || entry.orderNo != orderNo {
		entry = &dpcEntry{deviceID: deviceID, port1Based: port1Based, orderNo: orderNo, firstChargingAt: observedAt}
		d.entries[key] = entry
	}
	// 更新最近观测功率
	entry.lastObservedW = realtimePowerW
	d.mu.Unlock()

	// 峰值保持期
	if d.cfg.PeakHoldSeconds > 0 && observedAt.Sub(entry.firstChargingAt) < time.Duration(d.cfg.PeakHoldSeconds)*time.Second {
		return
	}

	// 调整频率
	if !entry.lastAdjustAt.IsZero() && observedAt.Sub(entry.lastAdjustAt) < time.Duration(d.cfg.StepIntervalSeconds)*time.Second {
		return
	}

	// 计算新的过载功率目标
	lastOver := entry.lastOverloadW
	if lastOver == 0 {
		// 首次设置：以当前功率为基准上浮10%，避免立刻触发降功
		lastOver = int(math.Max(float64(realtimePowerW)*1.1, float64(realtimePowerW+20)))
	}

	step := d.cfg.StepPercent
	if step <= 0 || step >= 1 {
		step = 0.1
	}
	minW := d.cfg.MinPowerW
	if minW <= 0 {
		minW = 80
	}

	target := int(math.Max(float64(lastOver)*(1.0-step), float64(minW)))

	// 防抖：变化阈值
	if abs(target-lastOver) < d.cfg.ChangeThresholdW {
		return
	}

	// 下发 0x82 更新过载功率（最大充电时长不修改=0）
	dg := GetGlobalDeviceGateway()
	if dg == nil {
		return
	}
	if err := dg.UpdateChargingOverloadPower(deviceID, uint8(port1Based), orderNo, uint16(target), 0); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"port":     port1Based,
			"orderNo":  orderNo,
			"fromW":    lastOver,
			"toW":      target,
			"error":    err.Error(),
		}).Warn("智能降功率：下发失败")
		return
	}

	entry.lastOverloadW = target
	entry.lastAdjustAt = observedAt
	entry.lastTargetW = target
	entry.lastTargetAt = observedAt

	logger.WithFields(logrus.Fields{
		"deviceID":  deviceID,
		"port":      port1Based,
		"orderNo":   orderNo,
		"realtimeW": realtimePowerW,
		"targetW":   target,
	}).Info("智能降功率：已更新过载功率")

	// 简易对账：若宽限时间后观测功率仍显著高于目标，则重发一次
	go func(dev string, p1 int, ord string, tgt int, _ time.Time) {
		grace := time.Duration(d.cfg.StepIntervalSeconds/2)*time.Second + 10*time.Second
		timer := time.NewTimer(grace)
		defer timer.Stop()
		<-timer.C

		d.mu.Lock()
		e := d.entries[makeKey(dev, p1)]
		if e != nil && e.orderNo == ord && e.lastTargetW == tgt && !e.resentOnce {
			margin := 10 // 瓦
			if e.lastObservedW > tgt+margin {
				d.mu.Unlock()
				if err := dg.UpdateChargingOverloadPower(dev, uint8(p1), ord, uint16(tgt), 0); err == nil {
					logger.WithFields(logrus.Fields{
						"deviceID": dev,
						"port":     p1,
						"orderNo":  ord,
						"targetW":  tgt,
					}).Warn("智能降功率：对账重发0x82一次")
					d.mu.Lock()
					e.resentOnce = true
					d.mu.Unlock()
				}
				return
			}
		}
		if e != nil {
			d.mu.Unlock()
		}
	}(deviceID, port1Based, orderNo, target, observedAt)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
