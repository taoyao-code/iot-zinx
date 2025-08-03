package tests

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestConcurrency 并发场景测试套件
// 测试多设备同时连接、多客户端同时请求等并发场景
func TestConcurrency(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("并发TCP连接测试", func(t *testing.T) {
		testConcurrentTCPConnections(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("并发HTTP请求测试", func(t *testing.T) {
		testConcurrentHTTPRequests(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("并发协议交互测试", func(t *testing.T) {
		testConcurrentProtocolInteractions(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("连接池压力测试", func(t *testing.T) {
		testConnectionPoolStress(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 打印测试摘要
	suite.PrintSummary()
}

// testConcurrentTCPConnections 并发TCP连接测试
func testConcurrentTCPConnections(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	concurrentNum := suite.GetConcurrentNum()
	var successCount, errorCount int64
	var wg sync.WaitGroup

	// 启动并发连接
	for i := 0; i < concurrentNum; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 建立TCP连接
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("并发连接%d失败: %v", id, err)
				return
			}

			// 保持连接一段时间
			time.Sleep(100 * time.Millisecond)

			// 关闭连接
			connHelper.CloseConnection(conn)
			atomic.AddInt64(&successCount, 1)
			t.Logf("并发连接%d成功", id)
		}(i)
	}

	// 等待所有连接完成
	wg.Wait()

	// 记录测试结果
	success := successCount > 0
	suite.RecordTestResult("并发TCP连接", "并发测试", success, time.Since(start), nil,
		fmt.Sprintf("成功:%d, 失败:%d", successCount, errorCount), map[string]int64{
			"success_count": successCount,
			"error_count":   errorCount,
		})

	// 断言验证
	assertHelper.AssertConcurrentResults(t, successCount, errorCount, concurrentNum, 60.0, "并发TCP连接")
}

// testConcurrentHTTPRequests 并发HTTP请求测试
func testConcurrentHTTPRequests(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	concurrentNum := suite.GetConcurrentNum()
	var successCount, errorCount int64
	var wg sync.WaitGroup

	// 创建HTTP客户端
	client := &http.Client{Timeout: suite.Timeout}

	// 启动并发HTTP请求
	for i := 0; i < concurrentNum; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 发送HTTP请求
			resp, err := client.Get(suite.HTTPBaseURL + "/health")
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("并发HTTP请求%d失败: %v", id, err)
				return
			}

			if resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					atomic.AddInt64(&successCount, 1)
					t.Logf("并发HTTP请求%d成功: %d", id, resp.StatusCode)
				} else {
					atomic.AddInt64(&errorCount, 1)
					t.Logf("并发HTTP请求%d状态异常: %d", id, resp.StatusCode)
				}
			}
		}(i)
	}

	// 等待所有请求完成
	wg.Wait()

	// 记录测试结果
	success := successCount > 0
	suite.RecordTestResult("并发HTTP请求", "并发测试", success, time.Since(start), nil,
		fmt.Sprintf("成功:%d, 失败:%d", successCount, errorCount), map[string]int64{
			"success_count": successCount,
			"error_count":   errorCount,
		})

	// 断言验证
	assertHelper.AssertConcurrentResults(t, successCount, errorCount, concurrentNum, 80.0, "并发HTTP请求")
}

// testConcurrentProtocolInteractions 并发协议交互测试
func testConcurrentProtocolInteractions(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	concurrentNum := 3 // 减少并发数，避免服务器压力过大
	var successCount, errorCount int64
	var wg sync.WaitGroup

	// 启动并发协议交互
	for i := 0; i < concurrentNum; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 建立连接
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("并发协议交互%d连接失败: %v", id, err)
				return
			}
			defer connHelper.CloseConnection(conn)

			// 发送协议数据
			deviceID := uint32(0x04A228CD + id) // 使用不同的设备ID
			messageID := uint16(0x0801 + id)

			// 发送设备注册包
			registerPacket := protocolHelper.BuildDeviceRegisterPacket(deviceID, messageID)
			err = connHelper.SendProtocolData(conn, registerPacket, fmt.Sprintf("并发注册%d", id))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("并发协议交互%d发送失败: %v", id, err)
				return
			}

			// 发送心跳包
			heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID+1)
			err = connHelper.SendProtocolData(conn, heartbeatPacket, fmt.Sprintf("并发心跳%d", id))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("并发协议交互%d心跳失败: %v", id, err)
				return
			}

			atomic.AddInt64(&successCount, 1)
			t.Logf("并发协议交互%d成功", id)
		}(i)
	}

	// 等待所有交互完成
	wg.Wait()

	// 记录测试结果
	success := successCount > 0
	suite.RecordTestResult("并发协议交互", "并发测试", success, time.Since(start), nil,
		fmt.Sprintf("成功:%d, 失败:%d", successCount, errorCount), map[string]int64{
			"success_count": successCount,
			"error_count":   errorCount,
		})

	// 断言验证
	assertHelper.AssertConcurrentResults(t, successCount, errorCount, concurrentNum, 60.0, "并发协议交互")
}

// testConnectionPoolStress 连接池压力测试
func testConnectionPoolStress(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	poolSize := 10
	var successCount, errorCount int64

	// 创建连接池
	connections, err := connHelper.CreateConnectionPool(suite.TCPAddress, poolSize)
	if err != nil {
		suite.RecordTestResult("连接池压力测试", "并发测试", false, time.Since(start), err, "连接池创建失败", nil)
		assertHelper.AssertNoError(t, err, "连接池创建")
		return
	}

	t.Logf("成功创建%d个连接的连接池", len(connections))

	// 并发使用连接池
	var wg sync.WaitGroup
	for i, conn := range connections {
		wg.Add(1)
		go func(id int, connection interface{}) {
			defer wg.Done()

			// 模拟使用连接发送数据
			deviceID := uint32(0x04A228CD + id)
			messageID := uint16(0x0801 + id)

			heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)
			err := connHelper.SendProtocolData(connection.(net.Conn), heartbeatPacket, fmt.Sprintf("连接池测试%d", id))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("连接池连接%d发送失败: %v", id, err)
			} else {
				atomic.AddInt64(&successCount, 1)
				t.Logf("连接池连接%d发送成功", id)
			}
		}(i, conn)
	}

	// 等待所有操作完成
	wg.Wait()

	// 关闭连接池
	closeErrors := connHelper.CloseConnectionPool(connections)
	if len(closeErrors) > 0 {
		t.Logf("关闭连接池时有%d个错误", len(closeErrors))
	}

	// 记录测试结果
	success := successCount > 0
	suite.RecordTestResult("连接池压力测试", "并发测试", success, time.Since(start), nil,
		fmt.Sprintf("连接池大小:%d, 成功:%d, 失败:%d", poolSize, successCount, errorCount), map[string]interface{}{
			"pool_size":     poolSize,
			"success_count": successCount,
			"error_count":   errorCount,
			"close_errors":  len(closeErrors),
		})

	// 断言验证
	assertHelper.AssertConcurrentResults(t, successCount, errorCount, poolSize, 70.0, "连接池压力测试")
}

// BenchmarkConcurrentTCPConnections 并发TCP连接性能基准测试
func BenchmarkConcurrentTCPConnections(b *testing.B) {
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

// BenchmarkConcurrentHTTPRequests 并发HTTP请求性能基准测试
func BenchmarkConcurrentHTTPRequests(b *testing.B) {
	suite := common.NewTestSuite(common.DefaultTestConfig())
	client := &http.Client{Timeout: suite.Timeout}
	url := suite.HTTPBaseURL + "/health"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(url)
			if err == nil && resp != nil {
				resp.Body.Close()
			}
		}
	})
}
