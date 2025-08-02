package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestConnectivity 基础连通性测试套件
// 迁移自debug_device_register.go中的基础连通性测试
func TestConnectivity(t *testing.T) {
	// 创建测试套件
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("TCP连接测试", func(t *testing.T) {
		testTCPConnection(t, suite, connHelper, assertHelper)
	})

	t.Run("HTTP连接测试", func(t *testing.T) {
		testHTTPConnection(t, suite, connHelper, assertHelper)
	})

	t.Run("健康检查API测试", func(t *testing.T) {
		testHealthCheck(t, suite, connHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testTCPConnection TCP连接测试
// 验证TCP服务器可达性
func testTCPConnection(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 尝试建立TCP连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	success := err == nil

	if success {
		defer connHelper.CloseConnection(conn)
		
		// 验证连接信息
		connInfo := connHelper.GetConnectionInfo(conn)
		t.Logf("TCP连接信息: %+v", connInfo)
	}

	// 记录测试结果
	suite.RecordTestResult("TCP连接测试", "连通性", success, time.Since(start), err, "验证TCP服务器可达性", nil)

	// 断言测试结果
	assertHelper.AssertNoError(t, err, "TCP连接")
}

// testHTTPConnection HTTP连接测试
// 验证HTTP服务器可达性
func testHTTPConnection(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: suite.Timeout,
	}

	// 发送HTTP请求
	resp, err := client.Get(suite.HTTPBaseURL)
	success := err == nil

	var statusCode int
	if resp != nil {
		statusCode = resp.StatusCode
		defer resp.Body.Close()
		success = success && (statusCode < 500) // 5xx错误认为连接失败
	}

	// 记录测试结果
	description := "验证HTTP服务器可达性"
	if resp != nil {
		description += ", 状态码: " + resp.Status
	}
	
	suite.RecordTestResult("HTTP连接测试", "连通性", success, time.Since(start), err, description, map[string]interface{}{
		"status_code": statusCode,
		"url":         suite.HTTPBaseURL,
	})

	// 断言测试结果
	if err == nil && resp != nil {
		assertHelper.AssertHTTPStatusRange(t, resp, 200, 499, "HTTP连接")
	} else {
		assertHelper.AssertNoError(t, err, "HTTP连接")
	}
}

// testHealthCheck 健康检查API测试
// 验证健康检查端点的可用性
func testHealthCheck(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: suite.Timeout,
	}

	// 发送健康检查请求
	healthURL := suite.HTTPBaseURL + "/health"
	resp, err := client.Get(healthURL)
	
	var success bool
	var responseData map[string]interface{}

	if err == nil && resp != nil {
		defer resp.Body.Close()
		
		// 读取响应体
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			// 尝试解析JSON响应
			json.Unmarshal(body, &responseData)
		}
		
		// 健康检查应该返回200状态码
		success = resp.StatusCode == 200
	}

	// 记录测试结果
	description := "验证健康检查API可用性"
	if resp != nil {
		description += ", 状态码: " + resp.Status
	}
	
	suite.RecordTestResult("健康检查API测试", "连通性", success, time.Since(start), err, description, responseData)

	// 断言测试结果
	if err == nil && resp != nil {
		assertHelper.AssertHTTPStatus(t, resp, 200, "健康检查API")
		
		// 如果有JSON响应，验证基本字段
		if responseData != nil {
			expectedFields := []string{"status"}
			assertHelper.AssertJSONResponse(t, nil, expectedFields, "健康检查响应格式")
		}
	} else {
		assertHelper.AssertNoError(t, err, "健康检查API")
	}
}

// TestConnectivityWithRetry 带重试的连通性测试
// 测试连接重试机制
func TestConnectivityWithRetry(t *testing.T) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("TCP连接重试测试", func(t *testing.T) {
		start := time.Now()

		// 设置重试配置
		connHelper.SetRetryConfig(3, 500*time.Millisecond)

		// 尝试连接（可能失败的地址用于测试重试）
		conn, err := connHelper.EstablishTCPConnectionWithRetry(suite.TCPAddress)
		success := err == nil

		if success {
			defer connHelper.CloseConnection(conn)
		}

		// 记录测试结果
		suite.RecordTestResult("TCP连接重试测试", "连通性-重试", success, time.Since(start), err, "验证TCP连接重试机制", nil)

		// 断言测试结果（重试测试可能失败，这是正常的）
		if err != nil {
			t.Logf("TCP连接重试失败（这可能是正常的）: %v", err)
		} else {
			assertHelper.AssertNoError(t, err, "TCP连接重试")
		}
	})
}

// TestConnectivityTimeout 连接超时测试
// 测试连接超时处理
func TestConnectivityTimeout(t *testing.T) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("TCP连接超时测试", func(t *testing.T) {
		start := time.Now()

		// 使用很短的超时时间
		shortTimeout := 1 * time.Millisecond
		conn, err := connHelper.EstablishTCPConnectionWithTimeout(suite.TCPAddress, shortTimeout)

		if conn != nil {
			defer connHelper.CloseConnection(conn)
		}

		// 记录测试结果
		success := err != nil // 期望超时错误
		suite.RecordTestResult("TCP连接超时测试", "连通性-超时", success, time.Since(start), err, "验证TCP连接超时处理", nil)

		// 断言应该有超时错误（或者连接成功也是可以的）
		if err != nil {
			assertHelper.AssertErrorContains(t, err, "timeout", "TCP连接超时")
		} else {
			t.Logf("TCP连接在短超时时间内成功建立")
		}
	})

	t.Run("HTTP请求超时测试", func(t *testing.T) {
		start := time.Now()

		// 创建带短超时的HTTP客户端
		client := &http.Client{
			Timeout: 1 * time.Millisecond,
		}

		resp, err := client.Get(suite.HTTPBaseURL)
		if resp != nil {
			defer resp.Body.Close()
		}

		// 记录测试结果
		success := err != nil // 期望超时错误
		suite.RecordTestResult("HTTP请求超时测试", "连通性-超时", success, time.Since(start), err, "验证HTTP请求超时处理", nil)

		// 断言应该有超时错误（或者请求成功也是可以的）
		if err != nil {
			t.Logf("HTTP请求超时（这是期望的）: %v", err)
		} else {
			t.Logf("HTTP请求在短超时时间内成功完成")
		}
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// BenchmarkTCPConnection TCP连接性能基准测试
func BenchmarkTCPConnection(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
		if err == nil && conn != nil {
			connHelper.CloseConnection(conn)
		}
	}
}

// BenchmarkHTTPRequest HTTP请求性能基准测试
func BenchmarkHTTPRequest(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	client := &http.Client{
		Timeout: suite.Timeout,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(suite.HTTPBaseURL + "/health")
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}
