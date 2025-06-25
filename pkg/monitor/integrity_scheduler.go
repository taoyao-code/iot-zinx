package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// IntegrityScheduler 数据完整性检查调度器
type IntegrityScheduler struct {
	// 检查间隔
	checkInterval time.Duration

	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc

	// 等待组
	wg sync.WaitGroup

	// 是否正在运行
	running bool
	mutex   sync.Mutex

	// 依赖的管理器
	tcpMonitor         *TCPMonitor
	sessionManager     *SessionManager
	deviceGroupManager *DeviceGroupManager
}

// NewIntegrityScheduler 创建数据完整性检查调度器
func NewIntegrityScheduler(interval time.Duration) *IntegrityScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &IntegrityScheduler{
		checkInterval: interval,
		ctx:           ctx,
		cancel:        cancel,
		running:       false,
	}
}

// SetDependencies 设置依赖的管理器
func (is *IntegrityScheduler) SetDependencies(
	tcpMonitor *TCPMonitor,
	sessionManager *SessionManager,
	deviceGroupManager *DeviceGroupManager,
) {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	is.tcpMonitor = tcpMonitor
	is.sessionManager = sessionManager
	is.deviceGroupManager = deviceGroupManager

	logger.Info("IntegrityScheduler: 依赖管理器已设置")
}

// Start 启动定期检查
func (is *IntegrityScheduler) Start() error {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	if is.running {
		return nil
	}

	if is.tcpMonitor == nil || is.sessionManager == nil || is.deviceGroupManager == nil {
		logger.Error("IntegrityScheduler: 启动失败，依赖管理器未完全设置")
		return nil
	}

	is.running = true
	is.wg.Add(1)

	go is.schedulerLoop()

	logger.WithField("interval", is.checkInterval.String()).Info("IntegrityScheduler: 数据完整性检查调度器已启动")
	return nil
}

// Stop 停止定期检查
func (is *IntegrityScheduler) Stop() {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	if !is.running {
		return
	}

	is.running = false
	is.cancel()
	is.wg.Wait()

	logger.Info("IntegrityScheduler: 数据完整性检查调度器已停止")
}

// schedulerLoop 调度器主循环
func (is *IntegrityScheduler) schedulerLoop() {
	defer is.wg.Done()

	ticker := time.NewTicker(is.checkInterval)
	defer ticker.Stop()

	// 启动后立即执行一次检查
	is.performIntegrityCheck("startup")

	for {
		select {
		case <-is.ctx.Done():
			logger.Debug("IntegrityScheduler: 调度器循环已停止")
			return
		case <-ticker.C:
			is.performIntegrityCheck("scheduled")
		}
	}
}

// performIntegrityCheck 执行完整性检查
func (is *IntegrityScheduler) performIntegrityCheck(trigger string) {
	startTime := time.Now()

	logFields := logrus.Fields{
		"trigger":   trigger,
		"timestamp": startTime.Format("2006-01-02 15:04:05"),
	}

	logger.WithFields(logFields).Info("IntegrityScheduler: 开始数据完整性检查")

	var totalIssues []string
	checkResults := make(map[string]int)

	// 1. TCPMonitor 完整性检查
	// 注意：TCPMonitor 重构后暂时移除了 integrityChecker，这里跳过检查
	if is.tcpMonitor != nil {
		// TODO: 实现 TCPMonitor 的完整性检查方法
		checkResults["tcpMonitor"] = 0
	}

	// 2. SessionManager 完整性检查
	if is.sessionManager != nil {
		issues := is.sessionManager.CheckSessionIntegrity("scheduled-session")
		checkResults["sessionManager"] = len(issues)
		totalIssues = append(totalIssues, issues...)
	}

	// 3. DeviceGroupManager 完整性检查
	if is.deviceGroupManager != nil {
		issues := is.deviceGroupManager.CheckGroupIntegrity("scheduled-group")
		checkResults["deviceGroupManager"] = len(issues)
		totalIssues = append(totalIssues, issues...)
	}

	// 4. 清理僵尸设备组
	var cleanedZombieGroups int
	if is.deviceGroupManager != nil {
		cleanedZombieGroups = is.deviceGroupManager.CleanupZombieGroups("scheduled-cleanup")
		checkResults["cleanedZombieGroups"] = cleanedZombieGroups
	}

	elapsed := time.Since(startTime)

	// 记录检查结果
	resultFields := logFields
	resultFields["elapsedMs"] = elapsed.Milliseconds()
	resultFields["totalIssues"] = len(totalIssues)
	resultFields["checkResults"] = checkResults
	resultFields["cleanedZombieGroups"] = cleanedZombieGroups

	if len(totalIssues) > 0 {
		resultFields["issues"] = totalIssues
		logger.WithFields(resultFields).Error("IntegrityScheduler: 数据完整性检查发现问题")

		// 🔧 触发告警（可以在这里集成告警系统）
		is.triggerAlert(totalIssues, checkResults)
	} else {
		logger.WithFields(resultFields).Info("IntegrityScheduler: 数据完整性检查通过")
	}
}

// triggerAlert 触发告警
func (is *IntegrityScheduler) triggerAlert(issues []string, checkResults map[string]int) {
	// 🔧 这里可以集成具体的告警系统，如邮件、短信、钉钉等
	logger.WithFields(logrus.Fields{
		"alertType":    "DATA_INTEGRITY_VIOLATION",
		"issueCount":   len(issues),
		"issues":       issues,
		"checkResults": checkResults,
		"severity":     "HIGH",
	}).Error("IntegrityScheduler: 数据完整性告警触发")

	// 示例：可以在这里添加具体的告警逻辑
	// - 发送邮件通知
	// - 调用告警API
	// - 写入告警日志文件
	// - 触发监控系统告警
}

// GetStatus 获取调度器状态
func (is *IntegrityScheduler) GetStatus() map[string]interface{} {
	is.mutex.Lock()
	defer is.mutex.Unlock()

	return map[string]interface{}{
		"running":       is.running,
		"checkInterval": is.checkInterval.String(),
		"dependencies": map[string]bool{
			"tcpMonitor":         is.tcpMonitor != nil,
			"sessionManager":     is.sessionManager != nil,
			"deviceGroupManager": is.deviceGroupManager != nil,
		},
	}
}

// 全局调度器实例
var (
	globalIntegrityScheduler     *IntegrityScheduler
	globalIntegritySchedulerOnce sync.Once
)

// GetGlobalIntegrityScheduler 获取全局数据完整性检查调度器
func GetGlobalIntegrityScheduler() *IntegrityScheduler {
	globalIntegritySchedulerOnce.Do(func() {
		// 默认每30分钟检查一次
		globalIntegrityScheduler = NewIntegrityScheduler(30 * time.Minute)
		logger.Info("IntegrityScheduler: 全局数据完整性检查调度器已初始化")
	})
	return globalIntegrityScheduler
}
