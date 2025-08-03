package tests

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestStress 压力测试套件
// 测试系统在高负载下的性能和稳定性
func TestStress(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("高频心跳压力测试", func(t *testing.T) {
		testHighFrequencyHeartbeat(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("大量连接压力测试", func(t *testing.T) {
		testMassiveConnections(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("长期稳定性测试", func(t *testing.T) {
		testLongTermStability(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("混合负载压力测试", func(t *testing.T) {
		testMixedLoadStress(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testHighFrequencyHeartbeat 高频心跳压力测试
func testHighFrequencyHeartbeat(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("高频心跳压力", "压力测试", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 高频发送心跳包
	deviceID := uint32(0x04A228CD)
	heartbeatCount := 100 // 发送100个心跳包
	interval := 50 * time.Millisecond // 50ms间隔
	successCount := 0
	errorCount := 0

	t.Logf("开始高频心跳测试：%d个心跳包，间隔%v", heartbeatCount, interval)

	for i := 0; i < heartbeatCount; i++ {
		messageID := uint16(0x1000 + i)
		heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)

		err := connHelper.SendProtocolData(conn, heartbeatPacket, fmt.Sprintf("心跳%d", i+1))
		if err != nil {
			errorCount++
			if errorCount <= 5 { // 只记录前5个错误
				t.Logf("心跳%d发送失败: %v", i+1, err)
			}
		} else {
			successCount++
			if (i+1)%20 == 0 {
				t.Logf("已发送%d个心跳包", i+1)
			}
		}

		time.Sleep(interval)
	}

	// 记录测试结果
	success := successCount > heartbeatCount*8/10 // 80%成功率
	suite.RecordTestResult("高频心跳压力", "压力测试", success, time.Since(start), nil,
		fmt.Sprintf("发送%d个心跳包，成功%d，失败%d", heartbeatCount, successCount, errorCount), map[string]interface{}{
			"total_count":   heartbeatCount,
			"success_count": successCount,
			"error_count":   errorCount,
			"success_rate":  float64(successCount) / float64(heartbeatCount) * 100,
			"interval":      interval.String(),
		})

	assertHelper.AssertTrue(t, success, "高频心跳压力测试")
	t.Logf("高频心跳测试完成：成功率%.1f%%", float64(successCount)/float64(heartbeatCount)*100)
}

// testMassiveConnections 大量连接压力测试
func testMassiveConnections(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 大量并发连接
	maxConnections := 100 // 限制连接数
	var successCount, errorCount int64
	var wg sync.WaitGroup

	t.Logf("开始大量连接测试：%d个并发连接", maxConnections)

	// 启动并发连接
	for i := 0; i < maxConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 建立连接
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			// 发送一个心跳包
			deviceID := uint32(0x04A228CD + id)
			messageID := uint16(0x2000 + id)
			heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)

			err = connHelper.SendProtocolData(conn, heartbeatPacket, fmt.Sprintf("大量连接测试%d", id))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}

			// 保持连接一段时间
			time.Sleep(100 * time.Millisecond)

			// 关闭连接
			connHelper.CloseConnection(conn)
		}(i)

		// 控制连接建立速度
		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 等待所有连接完成
	wg.Wait()

	// 记录测试结果
	success := successCount > int64(maxConnections)*6/10 // 60%成功率
	suite.RecordTestResult("大量连接压力", "压力测试", success, time.Since(start), nil,
		fmt.Sprintf("%d个并发连接，成功%d，失败%d", maxConnections, successCount, errorCount), map[string]interface{}{
			"max_connections": maxConnections,
			"success_count":   successCount,
			"error_count":     errorCount,
			"success_rate":    float64(successCount) / float64(maxConnections) * 100,
		})

	assertHelper.AssertConcurrentResults(t, successCount, errorCount, maxConnections, 60.0, "大量连接压力测试")
	t.Logf("大量连接测试完成：成功率%.1f%%", float64(successCount)/float64(maxConnections)*100)
}

// testLongTermStability 长期稳定性测试
func testLongTermStability(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 长期稳定性测试（缩短时间以适应测试环境）
	testDuration := 30 * time.Second // 30秒的长期测试
	heartbeatInterval := 1 * time.Second
	var successCount, errorCount int64

	t.Logf("开始长期稳定性测试：持续%v，心跳间隔%v", testDuration, heartbeatInterval)

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("长期稳定性", "压力测试", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 启动心跳发送
	deviceID := uint32(0x04A228CD)
	messageID := uint16(0x3000)
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	timeout := time.After(testDuration)
	heartbeatCount := 0

	for {
		select {
		case <-timeout:
			t.Log("长期稳定性测试时间到")
			goto TestComplete

		case <-ticker.C:
			heartbeatCount++
			currentMessageID := messageID + uint16(heartbeatCount)
			heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, currentMessageID)

			err := connHelper.SendProtocolData(conn, heartbeatPacket, fmt.Sprintf("长期心跳%d", heartbeatCount))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("长期心跳%d失败: %v", heartbeatCount, err)
			} else {
				atomic.AddInt64(&successCount, 1)
				if heartbeatCount%10 == 0 {
					t.Logf("已发送%d个长期心跳", heartbeatCount)
				}
			}
		}
	}

TestComplete:
	// 记录测试结果
	success := successCount > 0 && errorCount < successCount
	suite.RecordTestResult("长期稳定性", "压力测试", success, time.Since(start), nil,
		fmt.Sprintf("持续%v，发送%d个心跳，成功%d，失败%d", testDuration, heartbeatCount, successCount, errorCount), map[string]interface{}{
			"test_duration":   testDuration.String(),
			"heartbeat_count": heartbeatCount,
			"success_count":   successCount,
			"error_count":     errorCount,
			"success_rate":    float64(successCount) / float64(heartbeatCount) * 100,
		})

	assertHelper.AssertTrue(t, success, "长期稳定性测试")
	t.Logf("长期稳定性测试完成：%d个心跳，成功率%.1f%%", heartbeatCount, float64(successCount)/float64(heartbeatCount)*100)
}

// testMixedLoadStress 混合负载压力测试
func testMixedLoadStress(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 混合负载：TCP连接 + HTTP请求
	tcpConnections := 20
	httpRequests := 50
	var tcpSuccess, tcpError, httpSuccess, httpError int64
	var wg sync.WaitGroup

	t.Logf("开始混合负载测试：%d个TCP连接 + %d个HTTP请求", tcpConnections, httpRequests)

	// TCP连接负载
	for i := 0; i < tcpConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				atomic.AddInt64(&tcpError, 1)
				return
			}
			defer connHelper.CloseConnection(conn)

			// 发送多个协议包
			deviceID := uint32(0x04A228CD + id)
			for j := 0; j < 3; j++ {
				messageID := uint16(0x4000 + id*10 + j)
				heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)

				err = connHelper.SendProtocolData(conn, heartbeatPacket, fmt.Sprintf("混合TCP%d-%d", id, j))
				if err != nil {
					atomic.AddInt64(&tcpError, 1)
				} else {
					atomic.AddInt64(&tcpSuccess, 1)
				}

				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	// HTTP请求负载
	client := &http.Client{Timeout: suite.Timeout}
	for i := 0; i < httpRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			resp, err := client.Get(suite.HTTPBaseURL + "/health")
			if err != nil {
				atomic.AddInt64(&httpError, 1)
				return
			}

			if resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					atomic.AddInt64(&httpSuccess, 1)
				} else {
					atomic.AddInt64(&httpError, 1)
				}
			}
		}(i)
	}

	// 等待所有负载完成
	wg.Wait()

	// 计算总体结果
	totalOperations := tcpConnections*3 + httpRequests
	totalSuccess := tcpSuccess + httpSuccess
	totalError := tcpError + httpError

	// 记录测试结果
	success := totalSuccess > int64(totalOperations)*6/10 // 60%成功率
	suite.RecordTestResult("混合负载压力", "压力测试", success, time.Since(start), nil,
		fmt.Sprintf("总操作%d，成功%d，失败%d", totalOperations, totalSuccess, totalError), map[string]interface{}{
			"tcp_connections": tcpConnections,
			"http_requests":   httpRequests,
			"tcp_success":     tcpSuccess,
			"tcp_error":       tcpError,
			"http_success":    httpSuccess,
			"http_error":      httpError,
			"total_success":   totalSuccess,
			"total_error":     totalError,
			"success_rate":    float64(totalSuccess) / float64(totalOperations) * 100,
		})

	assertHelper.AssertTrue(t, success, "混合负载压力测试")
	t.Logf("混合负载测试完成：总成功率%.1f%% (TCP: %.1f%%, HTTP: %.1f%%)",
		float64(totalSuccess)/float64(totalOperations)*100,
		float64(tcpSuccess)/float64(tcpConnections*3)*100,
		float64(httpSuccess)/float64(httpRequests)*100)
}

// BenchmarkStressTCPConnections TCP连接压力基准测试
func BenchmarkStressTCPConnections(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err == nil && conn != nil {
				connHelper.CloseConnection(conn)
			}
		}
	})
}

// BenchmarkStressProtocolSending 协议发送压力基准测试
func BenchmarkStressProtocolSending(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		b.Fatalf("连接失败: %v", err)
	}
	defer connHelper.CloseConnection(conn)

	deviceID := uint32(0x04A228CD)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageID := uint16(i)
		heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)
		connHelper.SendProtocolData(conn, heartbeatPacket, "基准测试")
	}
}
