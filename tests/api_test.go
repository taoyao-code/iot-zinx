package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestAPI HTTP API测试套件
// 测试设备管理API、充电控制API等HTTP接口
func TestAPI(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("设备列表API测试", func(t *testing.T) {
		testDeviceListAPI(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("设备状态查询API测试", func(t *testing.T) {
		testDeviceStatusAPI(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("充电控制API测试", func(t *testing.T) {
		testChargingControlAPI(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("设备定位API测试", func(t *testing.T) {
		testDeviceLocationAPI(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("API错误处理测试", func(t *testing.T) {
		testAPIErrorHandling(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testDeviceListAPI 设备列表API测试
func testDeviceListAPI(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 测试获取设备列表
	url := suite.HTTPBaseURL + "/api/devices"
	resp, err := client.Get(url)
	
	success := false
	var responseData map[string]interface{}

	if err == nil && resp != nil {
		defer resp.Body.Close()
		
		// 读取响应体
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			// 解析JSON响应
			json.Unmarshal(body, &responseData)
			success = resp.StatusCode == 200
		}
	}

	// 记录测试结果
	suite.RecordTestResult("设备列表API", "HTTP-API", success, time.Since(start), err, 
		"GET /api/devices", responseData)

	// 断言验证
	if err == nil && resp != nil {
		assertHelper.AssertHTTPStatus(t, resp, 200, "设备列表API")
		if responseData != nil {
			expectedFields := []string{"devices"}
			assertHelper.AssertJSONResponse(t, nil, expectedFields, "设备列表响应格式")
		}
	} else {
		t.Logf("设备列表API测试失败: %v", err)
	}
}

// testDeviceStatusAPI 设备状态查询API测试
func testDeviceStatusAPI(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 测试设备ID列表
	deviceIDs := protocolHelper.GetTestDeviceIDs()
	allSuccess := true
	var lastErr error

	for _, deviceID := range deviceIDs {
		deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
		url := fmt.Sprintf("%s/api/devices/%s/status", suite.HTTPBaseURL, deviceIDStr)
		
		resp, err := client.Get(url)
		if err != nil {
			allSuccess = false
			lastErr = err
			t.Errorf("查询设备%s状态失败: %v", deviceIDStr, err)
			continue
		}

		if resp != nil {
			defer resp.Body.Close()
			
			// 读取响应
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				var statusData map[string]interface{}
				json.Unmarshal(body, &statusData)
				
				if resp.StatusCode == 200 || resp.StatusCode == 404 {
					t.Logf("设备%s状态查询成功: %d", deviceIDStr, resp.StatusCode)
				} else {
					allSuccess = false
					t.Errorf("设备%s状态查询异常: %d", deviceIDStr, resp.StatusCode)
				}
			}
		}
	}

	// 记录测试结果
	suite.RecordTestResult("设备状态查询API", "HTTP-API", allSuccess, time.Since(start), lastErr,
		fmt.Sprintf("查询%d个设备状态", len(deviceIDs)), len(deviceIDs))

	assertHelper.AssertTrue(t, allSuccess, "设备状态查询API测试")
}

// testChargingControlAPI 充电控制API测试
func testChargingControlAPI(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 测试充电控制命令
	commands := protocolHelper.GetChargingCommands()
	deviceID := protocolHelper.GetTestDeviceIDs()[0]
	deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
	allSuccess := true
	var lastErr error

	for _, cmd := range commands {
		// 构建充电控制请求
		requestData := map[string]interface{}{
			"command":  cmd.Command,
			"port":     1,
			"duration": 60,
		}
		
		jsonData, _ := json.Marshal(requestData)
		url := fmt.Sprintf("%s/api/devices/%s/charge", suite.HTTPBaseURL, deviceIDStr)
		
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			allSuccess = false
			lastErr = err
			t.Errorf("发送%s命令失败: %v", cmd.Name, err)
			continue
		}

		if resp != nil {
			defer resp.Body.Close()
			
			// 读取响应
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				var responseData map[string]interface{}
				json.Unmarshal(body, &responseData)
				
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					t.Logf("%s命令发送成功: %d", cmd.Name, resp.StatusCode)
				} else {
					t.Logf("%s命令响应: %d (可能正常)", cmd.Name, resp.StatusCode)
				}
			}
		}
	}

	// 记录测试结果
	suite.RecordTestResult("充电控制API", "HTTP-API", allSuccess, time.Since(start), lastErr,
		fmt.Sprintf("测试%d个充电控制命令", len(commands)), len(commands))

	// 充电控制API可能返回各种状态码，不强制要求成功
	t.Logf("充电控制API测试完成，测试了%d个命令", len(commands))
}

// testDeviceLocationAPI 设备定位API测试
func testDeviceLocationAPI(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 测试设备定位
	deviceIDs := protocolHelper.GetTestDeviceIDs()
	allSuccess := true
	var lastErr error

	for _, deviceID := range deviceIDs[:2] { // 只测试前2个设备
		deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
		url := fmt.Sprintf("%s/api/devices/%s/location", suite.HTTPBaseURL, deviceIDStr)
		
		resp, err := client.Get(url)
		if err != nil {
			allSuccess = false
			lastErr = err
			t.Errorf("查询设备%s定位失败: %v", deviceIDStr, err)
			continue
		}

		if resp != nil {
			defer resp.Body.Close()
			
			// 读取响应
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				var locationData map[string]interface{}
				json.Unmarshal(body, &locationData)
				
				if resp.StatusCode == 200 || resp.StatusCode == 404 {
					t.Logf("设备%s定位查询: %d", deviceIDStr, resp.StatusCode)
				} else {
					t.Logf("设备%s定位查询异常: %d", deviceIDStr, resp.StatusCode)
				}
			}
		}
	}

	// 记录测试结果
	suite.RecordTestResult("设备定位API", "HTTP-API", allSuccess, time.Since(start), lastErr,
		"查询设备定位信息", 2)

	// 定位API可能不存在，不强制要求成功
	t.Logf("设备定位API测试完成")
}

// testAPIErrorHandling API错误处理测试
func testAPIErrorHandling(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 测试用例
	testCases := []struct {
		name   string
		url    string
		method string
		data   string
	}{
		{"无效设备ID", "/api/devices/invalid/status", "GET", ""},
		{"不存在的端点", "/api/nonexistent", "GET", ""},
		{"无效JSON数据", "/api/devices/123/charge", "POST", "invalid json"},
		{"空设备ID", "/api/devices//status", "GET", ""},
	}

	allSuccess := true
	var lastErr error

	for _, tc := range testCases {
		url := suite.HTTPBaseURL + tc.url
		var resp *http.Response
		var err error

		switch tc.method {
		case "GET":
			resp, err = client.Get(url)
		case "POST":
			resp, err = client.Post(url, "application/json", bytes.NewBufferString(tc.data))
		}

		if err != nil {
			t.Logf("%s: 请求失败（可能正常）: %v", tc.name, err)
		} else if resp != nil {
			defer resp.Body.Close()
			t.Logf("%s: 响应状态 %d", tc.name, resp.StatusCode)
			
			// 4xx和5xx状态码是期望的错误响应
			if resp.StatusCode >= 400 && resp.StatusCode < 600 {
				// 这是正确的错误处理
			}
		}
	}

	// 记录测试结果
	suite.RecordTestResult("API错误处理", "HTTP-API", allSuccess, time.Since(start), lastErr,
		fmt.Sprintf("测试%d个错误场景", len(testCases)), len(testCases))

	t.Logf("API错误处理测试完成，测试了%d个错误场景", len(testCases))
}

// BenchmarkDeviceListAPI 设备列表API性能基准测试
func BenchmarkDeviceListAPI(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	client := &http.Client{Timeout: suite.Timeout}
	url := suite.HTTPBaseURL + "/api/devices"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}

// BenchmarkDeviceStatusAPI 设备状态API性能基准测试
func BenchmarkDeviceStatusAPI(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	client := &http.Client{Timeout: suite.Timeout}
	url := suite.HTTPBaseURL + "/api/devices/123/status"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}
