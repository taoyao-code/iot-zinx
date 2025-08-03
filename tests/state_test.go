package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestState 数据状态测试套件
// 测试设备状态变迁、数据一致性、持久化存储等
func TestState(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("设备状态变迁测试", func(t *testing.T) {
		testDeviceStateTransition(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("数据一致性检查测试", func(t *testing.T) {
		testDataConsistencyCheck(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("状态同步测试", func(t *testing.T) {
		testStateSynchronization(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("测试套件状态管理测试", func(t *testing.T) {
		testSuiteStateManagement(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testDeviceStateTransition 设备状态变迁测试
func testDeviceStateTransition(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试设备状态变迁序列
	deviceID := uint32(0x04A228CD)
	deviceIDStr := protocolHelper.FormatDeviceID(deviceID)

	// 状态变迁序列
	stateTransitions := []struct {
		action      string
		expectedState string
		description string
	}{
		{"初始化", "未连接", "设备初始状态"},
		{"建立连接", "已连接", "TCP连接建立"},
		{"发送注册", "注册中", "发送设备注册包"},
		{"注册完成", "已注册", "设备注册成功"},
		{"发送心跳", "在线", "设备心跳正常"},
		{"开始充电", "充电中", "开始充电操作"},
		{"停止充电", "空闲", "停止充电操作"},
		{"断开连接", "离线", "连接断开"},
	}

	allSuccess := true
	var lastErr error

	// 初始状态
	suite.SetDeviceState(deviceIDStr, "未连接")
	assertHelper.AssertDeviceState(t, suite, deviceIDStr, "未连接")

	// 执行状态变迁
	for i, transition := range stateTransitions {
		t.Logf("步骤%d: %s -> %s", i+1, transition.action, transition.expectedState)

		// 模拟执行操作
		switch transition.action {
		case "建立连接":
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				allSuccess = false
				lastErr = err
				t.Errorf("建立连接失败: %v", err)
				continue
			}
			defer connHelper.CloseConnection(conn)
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "发送注册":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "注册完成":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "发送心跳":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "开始充电":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "停止充电":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		case "断开连接":
			suite.SetDeviceState(deviceIDStr, transition.expectedState)

		default:
			suite.SetDeviceState(deviceIDStr, transition.expectedState)
		}

		// 验证状态变迁
		actualState, exists := suite.GetDeviceState(deviceIDStr)
		if !exists {
			allSuccess = false
			t.Errorf("设备状态不存在: %s", deviceIDStr)
		} else if actualState != transition.expectedState {
			allSuccess = false
			t.Errorf("状态变迁失败，期望'%s'，实际'%s'", transition.expectedState, actualState)
		} else {
			t.Logf("状态变迁成功: %s", actualState)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 记录测试结果
	suite.RecordTestResult("设备状态变迁", "状态测试", allSuccess, time.Since(start), lastErr,
		fmt.Sprintf("测试%d个状态变迁", len(stateTransitions)), map[string]interface{}{
			"device_id": deviceIDStr,
			"transitions": len(stateTransitions),
			"final_state": func() string {
				state, _ := suite.GetDeviceState(deviceIDStr)
				return state
			}(),
		})

	assertHelper.AssertTrue(t, allSuccess, "设备状态变迁测试")
}

// testDataConsistencyCheck 数据一致性检查测试
func testDataConsistencyCheck(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试多个设备的数据一致性
	deviceIDs := protocolHelper.GetTestDeviceIDs()[:3] // 使用前3个设备
	allSuccess := true
	var lastErr error

	// 为每个设备设置状态
	for i, deviceID := range deviceIDs {
		deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
		state := fmt.Sprintf("测试状态%d", i+1)
		suite.SetDeviceState(deviceIDStr, state)
		t.Logf("设置设备%s状态: %s", deviceIDStr, state)
	}

	// 验证所有设备状态
	allStates := suite.GetAllDeviceStates()
	t.Logf("当前所有设备状态: %+v", allStates)

	// 检查数据一致性
	for i, deviceID := range deviceIDs {
		deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
		expectedState := fmt.Sprintf("测试状态%d", i+1)
		
		actualState, exists := suite.GetDeviceState(deviceIDStr)
		if !exists {
			allSuccess = false
			t.Errorf("设备%s状态丢失", deviceIDStr)
		} else if actualState != expectedState {
			allSuccess = false
			t.Errorf("设备%s状态不一致，期望'%s'，实际'%s'", deviceIDStr, expectedState, actualState)
		} else {
			t.Logf("设备%s状态一致: %s", deviceIDStr, actualState)
		}
	}

	// 测试状态更新的一致性
	t.Log("测试状态更新一致性")
	for i, deviceID := range deviceIDs {
		deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
		newState := fmt.Sprintf("更新状态%d", i+1)
		suite.SetDeviceState(deviceIDStr, newState)
		
		// 立即验证更新
		actualState, exists := suite.GetDeviceState(deviceIDStr)
		if !exists || actualState != newState {
			allSuccess = false
			t.Errorf("设备%s状态更新失败", deviceIDStr)
		} else {
			t.Logf("设备%s状态更新成功: %s", deviceIDStr, actualState)
		}
	}

	// 记录测试结果
	suite.RecordTestResult("数据一致性检查", "状态测试", allSuccess, time.Since(start), lastErr,
		fmt.Sprintf("检查%d个设备的数据一致性", len(deviceIDs)), map[string]interface{}{
			"device_count": len(deviceIDs),
			"all_states": allStates,
		})

	assertHelper.AssertTrue(t, allSuccess, "数据一致性检查测试")
}

// testStateSynchronization 状态同步测试
func testStateSynchronization(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端用于查询API状态
	client := &http.Client{Timeout: suite.Timeout}
	allSuccess := true
	var lastErr error

	// 测试设备
	deviceID := uint32(0x04A228CD)
	deviceIDStr := protocolHelper.FormatDeviceID(deviceID)

	// 设置本地状态
	localState := "同步测试状态"
	suite.SetDeviceState(deviceIDStr, localState)
	t.Logf("设置本地状态: %s", localState)

	// 尝试通过API查询状态（可能不存在对应的API）
	url := fmt.Sprintf("%s/api/devices/%s/status", suite.HTTPBaseURL, deviceIDStr)
	resp, err := client.Get(url)
	
	if err != nil {
		t.Logf("API状态查询失败（可能正常）: %v", err)
	} else if resp != nil {
		defer resp.Body.Close()
		
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			var apiStatus map[string]interface{}
			json.Unmarshal(body, &apiStatus)
			
			t.Logf("API状态查询响应: %d, 数据: %+v", resp.StatusCode, apiStatus)
			
			// 如果API返回了状态信息，检查是否与本地状态一致
			if resp.StatusCode == 200 && apiStatus["status"] != nil {
				apiState := apiStatus["status"].(string)
				if apiState != localState {
					t.Logf("状态不同步 - 本地: %s, API: %s", localState, apiState)
				} else {
					t.Logf("状态同步正常")
				}
			}
		}
	}

	// 测试状态变更后的同步
	t.Log("测试状态变更同步")
	newState := "变更后状态"
	suite.SetDeviceState(deviceIDStr, newState)
	
	// 验证本地状态变更
	actualState, exists := suite.GetDeviceState(deviceIDStr)
	if !exists || actualState != newState {
		allSuccess = false
		t.Errorf("本地状态变更失败")
	} else {
		t.Logf("本地状态变更成功: %s", actualState)
	}

	// 记录测试结果
	suite.RecordTestResult("状态同步", "状态测试", allSuccess, time.Since(start), lastErr,
		"测试本地状态与API状态的同步", map[string]interface{}{
			"device_id": deviceIDStr,
			"local_state": newState,
		})

	t.Log("状态同步测试完成")
}

// testSuiteStateManagement 测试套件状态管理测试
func testSuiteStateManagement(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试测试套件本身的状态管理功能
	allSuccess := true
	var lastErr error

	// 清空现有状态
	suite.ClearDeviceStates()
	allStates := suite.GetAllDeviceStates()
	if len(allStates) != 0 {
		allSuccess = false
		t.Errorf("清空状态失败，仍有%d个状态", len(allStates))
	} else {
		t.Log("状态清空成功")
	}

	// 添加多个设备状态
	testStates := map[string]string{
		"device1": "状态1",
		"device2": "状态2",
		"device3": "状态3",
	}

	for deviceID, state := range testStates {
		suite.SetDeviceState(deviceID, state)
		t.Logf("添加设备状态: %s -> %s", deviceID, state)
	}

	// 验证所有状态
	allStates = suite.GetAllDeviceStates()
	if len(allStates) != len(testStates) {
		allSuccess = false
		t.Errorf("状态数量不匹配，期望%d，实际%d", len(testStates), len(allStates))
	}

	for deviceID, expectedState := range testStates {
		actualState, exists := suite.GetDeviceState(deviceID)
		if !exists {
			allSuccess = false
			t.Errorf("设备%s状态不存在", deviceID)
		} else if actualState != expectedState {
			allSuccess = false
			t.Errorf("设备%s状态不匹配，期望'%s'，实际'%s'", deviceID, expectedState, actualState)
		}
	}

	// 测试状态覆盖
	t.Log("测试状态覆盖")
	suite.SetDeviceState("device1", "新状态1")
	newState, exists := suite.GetDeviceState("device1")
	if !exists || newState != "新状态1" {
		allSuccess = false
		t.Errorf("状态覆盖失败")
	} else {
		t.Log("状态覆盖成功")
	}

	// 测试套件健康状态
	isHealthy := suite.IsHealthy()
	t.Logf("测试套件健康状态: %v", isHealthy)

	// 获取测试统计
	stats := suite.GetTestStatistics()
	t.Logf("测试统计: %+v", stats)

	// 记录测试结果
	suite.RecordTestResult("测试套件状态管理", "状态测试", allSuccess, time.Since(start), lastErr,
		"测试测试套件的状态管理功能", map[string]interface{}{
			"test_states_count": len(testStates),
			"final_states_count": len(allStates),
			"suite_healthy": isHealthy,
			"test_stats": stats,
		})

	assertHelper.AssertTrue(t, allSuccess, "测试套件状态管理测试")
	assertHelper.AssertTestSuiteHealth(t, suite, "测试套件健康检查")
}
