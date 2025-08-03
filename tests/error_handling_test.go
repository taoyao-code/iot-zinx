package tests

import (
	"net"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestErrorHandling 错误处理测试套件
// 测试各种异常情况下的系统行为和恢复能力
func TestErrorHandling(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("网络超时处理测试", func(t *testing.T) {
		testNetworkTimeoutHandling(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("无效数据处理测试", func(t *testing.T) {
		testInvalidDataHandling(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("连接异常断开测试", func(t *testing.T) {
		testConnectionDropHandling(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("资源耗尽处理测试", func(t *testing.T) {
		testResourceExhaustionHandling(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("服务器错误恢复测试", func(t *testing.T) {
		testServerErrorRecovery(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testNetworkTimeoutHandling 网络超时处理测试
func testNetworkTimeoutHandling(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试用例：不同的超时场景
	timeoutCases := []struct {
		name    string
		timeout time.Duration
		expectTimeout bool
	}{
		{"极短超时", 1 * time.Millisecond, true},
		{"短超时", 10 * time.Millisecond, true},
		{"正常超时", 1 * time.Second, false},
		{"长超时", 10 * time.Second, false},
	}

	allSuccess := true
	var lastErr error

	for _, tc := range timeoutCases {
		t.Logf("测试%s: %v", tc.name, tc.timeout)

		// 尝试建立连接
		conn, err := connHelper.EstablishTCPConnectionWithTimeout(suite.TCPAddress, tc.timeout)
		
		if tc.expectTimeout {
			// 期望超时
			if err != nil {
				t.Logf("%s: 正确产生超时错误: %v", tc.name, err)
			} else {
				t.Logf("%s: 意外成功建立连接", tc.name)
				if conn != nil {
					connHelper.CloseConnection(conn)
				}
			}
		} else {
			// 期望成功
			if err != nil {
				allSuccess = false
				lastErr = err
				t.Errorf("%s: 意外失败: %v", tc.name, err)
			} else {
				t.Logf("%s: 正确建立连接", tc.name)
				if conn != nil {
					connHelper.CloseConnection(conn)
				}
			}
		}
	}

	// 记录测试结果
	suite.RecordTestResult("网络超时处理", "错误处理", allSuccess, time.Since(start), lastErr,
		"测试不同超时场景的处理", len(timeoutCases))

	t.Logf("网络超时处理测试完成，测试了%d个场景", len(timeoutCases))
}

// testInvalidDataHandling 无效数据处理测试
func testInvalidDataHandling(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("无效数据处理", "错误处理", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 测试用例：各种无效数据
	invalidDataCases := []struct {
		name string
		data []byte
	}{
		{"空数据", []byte{}},
		{"随机数据", []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
		{"超长数据", make([]byte, 10000)},
		{"NULL字节", []byte{0x00, 0x00, 0x00, 0x00}},
		{"非ASCII数据", []byte{0xFF, 0xFE, 0xFD, 0xFC}},
	}

	allSuccess := true
	var lastErr error

	for _, tc := range invalidDataCases {
		t.Logf("发送%s", tc.name)

		err := connHelper.SendProtocolData(conn, tc.data, tc.name)
		if err != nil {
			t.Logf("%s发送失败（可能正常）: %v", tc.name, err)
		} else {
			t.Logf("%s发送成功", tc.name)
		}

		// 等待一小段时间，让服务器处理
		time.Sleep(100 * time.Millisecond)
	}

	// 记录测试结果
	suite.RecordTestResult("无效数据处理", "错误处理", allSuccess, time.Since(start), lastErr,
		"测试服务器对无效数据的处理", len(invalidDataCases))

	t.Logf("无效数据处理测试完成，测试了%d种无效数据", len(invalidDataCases))
}

// testConnectionDropHandling 连接异常断开测试
func testConnectionDropHandling(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试连接突然断开的场景
	dropCases := []struct {
		name     string
		action   string
		duration time.Duration
	}{
		{"立即断开", "immediate", 0},
		{"短暂连接后断开", "short", 100 * time.Millisecond},
		{"发送数据后断开", "after_send", 500 * time.Millisecond},
	}

	allSuccess := true
	var lastErr error

	for _, tc := range dropCases {
		t.Logf("测试%s", tc.name)

		// 建立连接
		conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
		if err != nil {
			allSuccess = false
			lastErr = err
			t.Errorf("%s: 连接建立失败: %v", tc.name, err)
			continue
		}

		// 根据测试用例执行不同的操作
		switch tc.action {
		case "immediate":
			// 立即关闭
			connHelper.CloseConnection(conn)
			
		case "short":
			// 等待一段时间后关闭
			time.Sleep(tc.duration)
			connHelper.CloseConnection(conn)
			
		case "after_send":
			// 发送数据后关闭
			deviceID := uint32(0x04A228CD)
			messageID := uint16(0x0801)
			packet := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)
			
			err = connHelper.SendProtocolData(conn, packet, "断开前心跳")
			if err != nil {
				t.Logf("%s: 发送数据失败: %v", tc.name, err)
			}
			
			time.Sleep(tc.duration)
			connHelper.CloseConnection(conn)
		}

		t.Logf("%s: 连接已断开", tc.name)
	}

	// 记录测试结果
	suite.RecordTestResult("连接异常断开", "错误处理", allSuccess, time.Since(start), lastErr,
		"测试连接异常断开的处理", len(dropCases))

	t.Logf("连接异常断开测试完成，测试了%d种断开场景", len(dropCases))
}

// testResourceExhaustionHandling 资源耗尽处理测试
func testResourceExhaustionHandling(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试大量连接的情况
	maxConnections := 50 // 限制连接数，避免系统资源耗尽
	connections := make([]net.Conn, 0, maxConnections)
	successCount := 0
	errorCount := 0

	t.Logf("尝试建立%d个连接", maxConnections)

	// 尝试建立大量连接
	for i := 0; i < maxConnections; i++ {
		conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
		if err != nil {
			errorCount++
			t.Logf("连接%d失败: %v", i+1, err)
			break
		} else {
			connections = append(connections, conn)
			successCount++
			if i%10 == 0 {
				t.Logf("已建立%d个连接", i+1)
			}
		}
	}

	t.Logf("成功建立%d个连接，失败%d个", successCount, errorCount)

	// 关闭所有连接
	for i, conn := range connections {
		connHelper.CloseConnection(conn)
		if i%10 == 0 {
			t.Logf("已关闭%d个连接", i+1)
		}
	}

	// 记录测试结果
	success := successCount > 0
	suite.RecordTestResult("资源耗尽处理", "错误处理", success, time.Since(start), nil,
		"测试大量连接的处理能力", map[string]int{
			"max_connections": maxConnections,
			"success_count":   successCount,
			"error_count":     errorCount,
		})

	assertHelper.AssertTrue(t, success, "资源耗尽处理测试")
}

// testServerErrorRecovery 服务器错误恢复测试
func testServerErrorRecovery(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试服务器在各种错误情况下的恢复能力
	recoveryCases := []struct {
		name        string
		description string
	}{
		{"连接后立即断开", "测试服务器处理突然断开的连接"},
		{"发送异常数据", "测试服务器处理异常协议数据"},
		{"频繁连接断开", "测试服务器处理频繁的连接建立和断开"},
	}

	allSuccess := true
	var lastErr error

	for _, tc := range recoveryCases {
		t.Logf("测试%s: %s", tc.name, tc.description)

		// 执行可能导致服务器错误的操作
		switch tc.name {
		case "连接后立即断开":
			for i := 0; i < 5; i++ {
				conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
				if err == nil {
					connHelper.CloseConnection(conn)
				}
			}

		case "发送异常数据":
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err == nil {
				// 发送各种异常数据
				malformedData := [][]byte{
					protocolHelper.BuildMalformedPacket("invalid_header"),
					protocolHelper.BuildMalformedPacket("wrong_length"),
					protocolHelper.BuildMalformedPacket("truncated"),
				}
				
				for _, data := range malformedData {
					connHelper.SendProtocolData(conn, data, "异常数据")
					time.Sleep(50 * time.Millisecond)
				}
				
				connHelper.CloseConnection(conn)
			}

		case "频繁连接断开":
			for i := 0; i < 10; i++ {
				conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
				if err == nil {
					time.Sleep(10 * time.Millisecond)
					connHelper.CloseConnection(conn)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

		// 等待服务器处理
		time.Sleep(500 * time.Millisecond)

		// 测试服务器是否仍然可用
		conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
		if err != nil {
			allSuccess = false
			lastErr = err
			t.Errorf("%s后服务器不可用: %v", tc.name, err)
		} else {
			t.Logf("%s后服务器仍然可用", tc.name)
			connHelper.CloseConnection(conn)
		}
	}

	// 记录测试结果
	suite.RecordTestResult("服务器错误恢复", "错误处理", allSuccess, time.Since(start), lastErr,
		"测试服务器的错误恢复能力", len(recoveryCases))

	assertHelper.AssertTrue(t, allSuccess, "服务器错误恢复测试")
}
