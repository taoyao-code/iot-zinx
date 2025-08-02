package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// TestSuite 统一测试套件基础类
// 提供测试配置、结果记录和状态管理功能
type TestSuite struct {
	// 服务配置
	HTTPBaseURL string
	TCPAddress  string
	Timeout     time.Duration

	// 测试管理
	testResults   []TestResult
	deviceStates  map[string]string
	mutex         sync.RWMutex
	concurrentNum int

	// 测试统计
	totalTests   int
	passedTests  int
	failedTests  int
	skippedTests int
}

// TestResult 测试结果结构
type TestResult struct {
	TestName     string        `json:"test_name"`
	TestType     string        `json:"test_type"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	Error        error         `json:"error,omitempty"`
	Description  string        `json:"description"`
	ResponseData interface{}   `json:"response_data,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// TestConfig 测试配置
type TestConfig struct {
	HTTPBaseURL   string
	TCPAddress    string
	Timeout       time.Duration
	ConcurrentNum int
	RetryCount    int
	RetryDelay    time.Duration
}

// NewTestSuite 创建新的测试套件实例
func NewTestSuite(config *TestConfig) *TestSuite {
	if config == nil {
		config = DefaultTestConfig()
	}

	return &TestSuite{
		HTTPBaseURL:   config.HTTPBaseURL,
		TCPAddress:    config.TCPAddress,
		Timeout:       config.Timeout,
		testResults:   make([]TestResult, 0),
		deviceStates:  make(map[string]string),
		concurrentNum: config.ConcurrentNum,
	}
}

// DefaultTestConfig 默认测试配置
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		HTTPBaseURL:   "http://localhost:7055",
		TCPAddress:    "localhost:7054",
		Timeout:       10 * time.Second,
		ConcurrentNum: 5,
		RetryCount:    3,
		RetryDelay:    1 * time.Second,
	}
}

// RecordTestResult 记录测试结果
func (ts *TestSuite) RecordTestResult(testName, testType string, success bool, duration time.Duration, err error, description string, responseData interface{}) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	result := TestResult{
		TestName:     testName,
		TestType:     testType,
		Success:      success,
		Duration:     duration,
		Error:        err,
		Description:  description,
		ResponseData: responseData,
		Timestamp:    time.Now(),
	}

	ts.testResults = append(ts.testResults, result)
	ts.totalTests++

	if success {
		ts.passedTests++
	} else {
		ts.failedTests++
	}
}

// GetTestResults 获取所有测试结果
func (ts *TestSuite) GetTestResults() []TestResult {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	// 返回副本以避免并发修改
	results := make([]TestResult, len(ts.testResults))
	copy(results, ts.testResults)
	return results
}

// GetTestStatistics 获取测试统计信息
func (ts *TestSuite) GetTestStatistics() map[string]interface{} {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	return map[string]interface{}{
		"total":   ts.totalTests,
		"passed":  ts.passedTests,
		"failed":  ts.failedTests,
		"skipped": ts.skippedTests,
		"success_rate": func() float64 {
			if ts.totalTests == 0 {
				return 0.0
			}
			return float64(ts.passedTests) / float64(ts.totalTests) * 100
		}(),
	}
}

// SetDeviceState 设置设备状态
func (ts *TestSuite) SetDeviceState(deviceID, state string) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.deviceStates[deviceID] = state
}

// GetDeviceState 获取设备状态
func (ts *TestSuite) GetDeviceState(deviceID string) (string, bool) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	state, exists := ts.deviceStates[deviceID]
	return state, exists
}

// GetAllDeviceStates 获取所有设备状态
func (ts *TestSuite) GetAllDeviceStates() map[string]string {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	// 返回副本以避免并发修改
	states := make(map[string]string)
	for k, v := range ts.deviceStates {
		states[k] = v
	}
	return states
}

// ClearTestResults 清空测试结果
func (ts *TestSuite) ClearTestResults() {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.testResults = make([]TestResult, 0)
	ts.totalTests = 0
	ts.passedTests = 0
	ts.failedTests = 0
	ts.skippedTests = 0
}

// ClearDeviceStates 清空设备状态
func (ts *TestSuite) ClearDeviceStates() {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.deviceStates = make(map[string]string)
}

// PrintSummary 打印测试摘要
func (ts *TestSuite) PrintSummary() {
	stats := ts.GetTestStatistics()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 测试摘要报告")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("总测试数: %d\n", stats["total"])
	fmt.Printf("通过: %d\n", stats["passed"])
	fmt.Printf("失败: %d\n", stats["failed"])
	fmt.Printf("跳过: %d\n", stats["skipped"])
	fmt.Printf("成功率: %.2f%%\n", stats["success_rate"])

	// 打印失败的测试
	failedResults := ts.getFailedResults()
	if len(failedResults) > 0 {
		fmt.Println("\n❌ 失败的测试:")
		for _, result := range failedResults {
			fmt.Printf("  - %s (%s): %v\n", result.TestName, result.TestType, result.Error)
		}
	}

	fmt.Println(strings.Repeat("=", 60))
}

// getFailedResults 获取失败的测试结果
func (ts *TestSuite) getFailedResults() []TestResult {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	var failed []TestResult
	for _, result := range ts.testResults {
		if !result.Success {
			failed = append(failed, result)
		}
	}
	return failed
}

// GetConcurrentNum 获取并发数
func (ts *TestSuite) GetConcurrentNum() int {
	return ts.concurrentNum
}

// SetConcurrentNum 设置并发数
func (ts *TestSuite) SetConcurrentNum(num int) {
	if num > 0 {
		ts.concurrentNum = num
	}
}

// IsHealthy 检查测试套件健康状态
func (ts *TestSuite) IsHealthy() bool {
	stats := ts.GetTestStatistics()
	successRate := stats["success_rate"].(float64)
	return successRate >= 80.0 // 成功率80%以上认为健康
}
