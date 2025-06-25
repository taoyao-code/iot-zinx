package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// IntegrityTestHelper 数据完整性测试助手
type IntegrityTestHelper struct {
	tcpMonitor         *TCPMonitor
	sessionManager     *SessionManager
	deviceGroupManager *DeviceGroupManager
}

// NewIntegrityTestHelper 创建测试助手
func NewIntegrityTestHelper(
	tcpMonitor *TCPMonitor,
	sessionManager *SessionManager,
	deviceGroupManager *DeviceGroupManager,
) *IntegrityTestHelper {
	return &IntegrityTestHelper{
		tcpMonitor:         tcpMonitor,
		sessionManager:     sessionManager,
		deviceGroupManager: deviceGroupManager,
	}
}

// RunComprehensiveIntegrityCheck 运行综合完整性检查
func (ith *IntegrityTestHelper) RunComprehensiveIntegrityCheck(context string) *IntegrityCheckResult {
	startTime := time.Now()

	result := &IntegrityCheckResult{
		Context:   context,
		StartTime: startTime,
		Issues:    make(map[string][]string),
	}

	logger.WithField("context", context).Info("IntegrityTestHelper: 开始综合完整性检查")

	// 1. TCPMonitor 完整性检查
	if ith.tcpMonitor != nil {
		// TODO: 实现 TCPMonitor 的完整性检查方法
		result.Issues["tcpMonitor"] = []string{}
		result.TCPMonitorIssues = 0
	}

	// 2. SessionManager 完整性检查
	if ith.sessionManager != nil {
		issues := ith.sessionManager.CheckSessionIntegrity(context + "-session")
		result.Issues["sessionManager"] = issues
		result.SessionManagerIssues = len(issues)
	}

	// 3. DeviceGroupManager 完整性检查
	if ith.deviceGroupManager != nil {
		issues := ith.deviceGroupManager.CheckGroupIntegrity(context + "-group")
		result.Issues["deviceGroupManager"] = issues
		result.DeviceGroupManagerIssues = len(issues)
	}

	// 4. 跨组件一致性检查
	crossIssues := ith.checkCrossComponentConsistency(context)
	result.Issues["crossComponent"] = crossIssues
	result.CrossComponentIssues = len(crossIssues)

	// 5. 统计总问题数
	result.TotalIssues = result.TCPMonitorIssues + result.SessionManagerIssues +
		result.DeviceGroupManagerIssues + result.CrossComponentIssues

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 记录检查结果
	logFields := logrus.Fields{
		"context":                  context,
		"duration":                 result.Duration.String(),
		"totalIssues":              result.TotalIssues,
		"tcpMonitorIssues":         result.TCPMonitorIssues,
		"sessionManagerIssues":     result.SessionManagerIssues,
		"deviceGroupManagerIssues": result.DeviceGroupManagerIssues,
		"crossComponentIssues":     result.CrossComponentIssues,
	}

	if result.TotalIssues > 0 {
		logger.WithFields(logFields).Error("IntegrityTestHelper: 综合完整性检查发现问题")
	} else {
		logger.WithFields(logFields).Info("IntegrityTestHelper: 综合完整性检查通过")
	}

	return result
}

// checkCrossComponentConsistency 检查跨组件一致性
func (ith *IntegrityTestHelper) checkCrossComponentConsistency(context string) []string {
	var issues []string

	// 获取所有设备会话
	allSessions := ith.sessionManager.GetAllSessions()

	for deviceID, session := range allSessions {
		// 检查设备在TCPMonitor中的映射
		if ith.tcpMonitor != nil {
			if conn, exists := ith.tcpMonitor.GetConnectionByDeviceId(deviceID); exists {
				// 检查连接ID是否与会话中的一致
				if session.ConnID != conn.GetConnID() {
					issues = append(issues, fmt.Sprintf("设备 %s 的会话连接ID (%d) 与TCPMonitor中的连接ID (%d) 不一致",
						deviceID, session.ConnID, conn.GetConnID()))
				}
			} else {
				// 如果设备在线但TCPMonitor中没有连接，这是问题
				if session.Status == constants.DeviceStatusOnline {
					issues = append(issues, fmt.Sprintf("设备 %s 状态为在线但在TCPMonitor中没有连接", deviceID))
				}
			}
		}

		// 检查设备在设备组中的一致性
		if session.ICCID != "" && ith.deviceGroupManager != nil {
			if groupSession, exists := ith.deviceGroupManager.GetDeviceFromGroup(session.ICCID, deviceID); exists {
				// 检查设备组中的会话是否与SessionManager中的一致
				if groupSession.SessionID != session.SessionID {
					issues = append(issues, fmt.Sprintf("设备 %s 在设备组中的会话ID (%s) 与SessionManager中的不一致 (%s)",
						deviceID, groupSession.SessionID, session.SessionID))
				}
			} else {
				issues = append(issues, fmt.Sprintf("设备 %s 有ICCID %s 但不在对应的设备组中", deviceID, session.ICCID))
			}
		}
	}

	return issues
}

// SimulateConcurrentOperations 模拟并发操作测试
func (ith *IntegrityTestHelper) SimulateConcurrentOperations(deviceCount int, operationCount int) *ConcurrencyTestResult {
	startTime := time.Now()

	result := &ConcurrencyTestResult{
		DeviceCount:    deviceCount,
		OperationCount: operationCount,
		StartTime:      startTime,
		Errors:         make([]string, 0),
	}

	logger.WithFields(logrus.Fields{
		"deviceCount":    deviceCount,
		"operationCount": operationCount,
	}).Info("IntegrityTestHelper: 开始并发操作测试")

	var wg sync.WaitGroup

	// 模拟并发设备注册/断开操作
	for i := 0; i < operationCount; i++ {
		wg.Add(1)

		go func(opIndex int) {
			defer wg.Done()

			deviceID := fmt.Sprintf("test-device-%d", opIndex%deviceCount)
			iccid := fmt.Sprintf("test-iccid-%d", opIndex%(deviceCount/2+1))

			// 模拟设备注册
			session := &DeviceSession{
				SessionID:     fmt.Sprintf("session-%d-%d", opIndex, time.Now().UnixNano()),
				DeviceID:      deviceID,
				ICCID:         iccid,
				Status:        constants.DeviceStatusOnline,
				ConnectedAt:   time.Now(),
				LastHeartbeat: time.Now(),
				ConnID:        uint64(opIndex + 1000),
			}

			// 添加到设备组
			if ith.deviceGroupManager != nil {
				ith.deviceGroupManager.AddDeviceToGroup(iccid, deviceID, session)
			}

			// 短暂等待
			time.Sleep(time.Millisecond * 10)

			// 模拟设备断开
			if ith.deviceGroupManager != nil {
				ith.deviceGroupManager.RemoveDeviceFromGroup(iccid, deviceID)
			}
		}(i)
	}

	// 等待所有操作完成
	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 执行完整性检查
	integrityResult := ith.RunComprehensiveIntegrityCheck("concurrency-test")
	result.IntegrityIssues = integrityResult.TotalIssues
	result.IntegrityDetails = integrityResult

	logger.WithFields(logrus.Fields{
		"duration":        result.Duration.String(),
		"integrityIssues": result.IntegrityIssues,
		"errors":          len(result.Errors),
	}).Info("IntegrityTestHelper: 并发操作测试完成")

	return result
}

// IntegrityCheckResult 完整性检查结果
type IntegrityCheckResult struct {
	Context                  string
	StartTime                time.Time
	EndTime                  time.Time
	Duration                 time.Duration
	TotalIssues              int
	TCPMonitorIssues         int
	SessionManagerIssues     int
	DeviceGroupManagerIssues int
	CrossComponentIssues     int
	Issues                   map[string][]string
}

// ConcurrencyTestResult 并发测试结果
type ConcurrencyTestResult struct {
	DeviceCount      int
	OperationCount   int
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	Errors           []string
	IntegrityIssues  int
	IntegrityDetails *IntegrityCheckResult
}

// PrintSummary 打印结果摘要
func (icr *IntegrityCheckResult) PrintSummary() {
	fmt.Printf("\n=== 数据完整性检查结果摘要 ===\n")
	fmt.Printf("检查上下文: %s\n", icr.Context)
	fmt.Printf("检查耗时: %s\n", icr.Duration.String())
	fmt.Printf("总问题数: %d\n", icr.TotalIssues)
	fmt.Printf("  - TCPMonitor问题: %d\n", icr.TCPMonitorIssues)
	fmt.Printf("  - SessionManager问题: %d\n", icr.SessionManagerIssues)
	fmt.Printf("  - DeviceGroupManager问题: %d\n", icr.DeviceGroupManagerIssues)
	fmt.Printf("  - 跨组件一致性问题: %d\n", icr.CrossComponentIssues)

	if icr.TotalIssues > 0 {
		fmt.Printf("\n详细问题列表:\n")
		for component, issues := range icr.Issues {
			if len(issues) > 0 {
				fmt.Printf("  %s:\n", component)
				for _, issue := range issues {
					fmt.Printf("    - %s\n", issue)
				}
			}
		}
	}
	fmt.Printf("================================\n\n")
}

// PrintSummary 打印并发测试结果摘要
func (ctr *ConcurrencyTestResult) PrintSummary() {
	fmt.Printf("\n=== 并发操作测试结果摘要 ===\n")
	fmt.Printf("设备数量: %d\n", ctr.DeviceCount)
	fmt.Printf("操作数量: %d\n", ctr.OperationCount)
	fmt.Printf("测试耗时: %s\n", ctr.Duration.String())
	fmt.Printf("错误数量: %d\n", len(ctr.Errors))
	fmt.Printf("完整性问题: %d\n", ctr.IntegrityIssues)

	if len(ctr.Errors) > 0 {
		fmt.Printf("\n错误列表:\n")
		for _, err := range ctr.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	if ctr.IntegrityDetails != nil && ctr.IntegrityDetails.TotalIssues > 0 {
		fmt.Printf("\n完整性检查详情:\n")
		ctr.IntegrityDetails.PrintSummary()
	}
	fmt.Printf("==============================\n\n")
}
