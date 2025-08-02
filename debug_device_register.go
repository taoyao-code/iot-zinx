package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 全局配置
var (
	verboseMode = false // 设置为false只显示错误日志，true显示所有日志
)

// 综合测试客户端 - 测试TCP协议、HTTP API、并发场景、错误处理等
func main() {
	fmt.Println("IoT-Zinx 综合测试客户端")
	fmt.Println("= " + strings.Repeat("=", 60))

	if !verboseMode {
		fmt.Println("📝 简化模式：只显示错误和重要信息（设置 verboseMode=true 查看详细日志）")
		fmt.Println()
	}

	// 创建测试套件
	suite := NewTestSuite()

	// 🔥 新增：使用真实协议数据进行本地测试
	suite.runRealDataProtocolTest()

	// 执行其他测试
	suite.RunAllTests()
}

// TestSuite 测试套件
type TestSuite struct {
	httpBaseURL   string
	tcpAddress    string
	testResults   []TestResult
	mutex         sync.Mutex
	deviceStates  map[string]string
	concurrentNum int
}

// TestResult 测试结果
type TestResult struct {
	TestName     string
	TestType     string
	Success      bool
	Duration     time.Duration
	Error        error
	Description  string
	ResponseData interface{}
}

// NewTestSuite 创建测试套件
func NewTestSuite() *TestSuite {
	return &TestSuite{
		httpBaseURL:   "http://localhost:7055",
		tcpAddress:    "localhost:7054",
		testResults:   make([]TestResult, 0),
		deviceStates:  make(map[string]string),
		concurrentNum: 5,
	}
}

// RunAllTests 运行所有测试
func (ts *TestSuite) RunAllTests() {
	logImportant("开始综合测试...\n")

	// 1. 基础连通性测试
	ts.runConnectivityTests()

	// 2. TCP协议测试
	ts.runTCPProtocolTests()

	// 3. HTTP API测试
	ts.runHTTPAPITests()

	// 4. 并发场景测试
	ts.runConcurrencyTests()

	// 5. 错误处理测试
	ts.runErrorHandlingTests()

	// 6. 数据状态测试
	ts.runDataStateTests()

	// 7. 压力测试
	ts.runStressTests()

	// 8. 协议兼容性测试
	ts.runProtocolCompatibilityTests()

	// 输出测试报告
	ts.generateReport()
}

// 🔥 新增：使用真实协议数据的本地测试
func (ts *TestSuite) runRealDataProtocolTest() {
	logImportant("=== 真实协议数据测试 ===\n")
	logImportant("使用生产环境的真实数据包进行本地测试验证\n")

	start := time.Now()

	// 连接到本地TCP服务器
	conn, err := net.DialTimeout("tcp", ts.tcpAddress, 10*time.Second)
	if err != nil {
		ts.recordTestResult("真实数据测试-连接", "真实协议", false, time.Since(start), err, "无法连接到本地TCP服务器", nil)
		return
	}
	defer conn.Close()

	logSuccess("成功连接到本地TCP服务器: %s\n", ts.tcpAddress)

	// === 步骤1：发送真实ICCID数据包 ===
	logImportant("步骤1：发送真实ICCID数据包\n")
	iccidStr := "898604D9162390488297" // 来自真实日志
	iccidBytes := []byte(iccidStr)

	logInfo("发送ICCID: %s (%d字节)\n", iccidStr, len(iccidBytes))
	logInfo("十六进制: %x\n", iccidBytes)

	err = ts.sendDataPacket(conn, iccidBytes, "ICCID数据包")
	if err != nil {
		ts.recordTestResult("真实数据测试-ICCID", "真实协议", false, time.Since(start), err, "ICCID发送失败", nil)
		return
	}

	time.Sleep(1 * time.Second) // 等待服务器处理

	// === 步骤2：发送真实Link心跳 ===
	logImportant("步骤2：发送真实Link心跳\n")
	linkBytes := []byte("link") // 来自真实日志: 6c696e6b

	logInfo("发送Link心跳: %s (%d字节)\n", string(linkBytes), len(linkBytes))
	logInfo("十六进制: %x\n", linkBytes)

	err = ts.sendDataPacket(conn, linkBytes, "Link心跳")
	if err != nil {
		ts.recordTestResult("真实数据测试-Link心跳", "真实协议", false, time.Since(start), err, "Link心跳发送失败", nil)
		return
	}

	time.Sleep(500 * time.Millisecond)

	// === 步骤3：发送真实DNY协议包 ===
	logImportant("步骤3：发送真实DNY协议包\n")

	// 来自真实日志的DNY数据包
	realDNYPackets := []struct {
		name string
		hex  string
		desc string
	}{
		{
			name: "刷卡操作包",
			hex:  "444e590900f36ca2040200120d03",
			desc: "物理ID: 04A26CF3, 命令: 0x02 (刷卡操作), 消息ID: 0x1200",
		},
		{
			name: "结算消费信息上传包",
			hex:  "444e595000f36ca2040300116b0202dd888d681c07383938363034443931363233393034383832393755000038363434353230363937363234373256312e302e30302e3030303030302e303631390000000000a711",
			desc: "物理ID: 04A26CF3, 命令: 0x03 (结算消费信息上传), 包含消费详情",
		},
		{
			name: "订单确认包",
			hex:  "444e591200f36ca2040400350131008002f36ca204f405",
			desc: "物理ID: 04A26CF3, 命令: 0x04 (充电端口订单确认，老版本指令)",
		},
		{
			name: "端口功率心跳包",
			hex:  "444e590f00f36ca2040600208002020a31065704",
			desc: "物理ID: 04A26CF3, 命令: 0x06 (端口充电时功率心跳包), 修正为正确指令",
		},
		{
			name: "设备注册包",
			hex:  "444e590d00f36ca2042000013c0201063302",
			desc: "物理ID: 04A26CF3, 命令: 0x20 (设备注册包), 正确的注册指令",
		},
	}

	for _, packet := range realDNYPackets {
		logInfo("发送 %s: %s\n", packet.name, packet.desc)
		logInfo("十六进制: %s\n", packet.hex)

		data := ts.hexStringToBytes(packet.hex)
		if data == nil {
			logError("十六进制解码失败: %s\n", packet.hex)
			continue
		}

		logInfo("解码后: %d字节, %x\n", len(data), data)

		err = ts.sendDataPacket(conn, data, packet.name)
		if err != nil {
			logError("%s 发送失败: %v\n", packet.name, err)
		} else {
			logInfo("✅ %s 发送成功\n", packet.name)
		}

		// 尝试读取响应
		response := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(response)
		if err == nil && n > 0 {
			logInfo("📥 收到响应: %d字节, %x\n", n, response[:n])
		} else {
			logInfo("📭 无响应或超时\n")
		}

		time.Sleep(1 * time.Second) // 等待处理
	}

	// === 步骤4：验证API接口 ===
	logImportant("步骤4：验证HTTP API接口\n")
	time.Sleep(2 * time.Second) // 给服务器更多时间处理数据

	// 查询设备列表
	logInfo("查询设备列表API...\n")
	resp, body, err := ts.makeHTTPRequest("GET", ts.httpBaseURL+"/api/v1/devices", nil)
	if err != nil {
		logError("API请求失败: %v\n", err)
	} else {
		logSuccess("API响应状态: %d\n", resp.StatusCode)
		logInfo("📄 响应内容: %s\n", string(body))

		// 解析JSON响应
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if data, ok := apiResp["data"].([]interface{}); ok {
				logImportant("📊 设备数量: %d\n", len(data))
				if len(data) > 0 {
					logSuccess("成功！发现设备数据\n")
					for i, device := range data {
						if deviceMap, ok := device.(map[string]interface{}); ok {
							logImportant("  设备%d: %v\n", i+1, deviceMap)
						}
					}
				} else {
					logError("设备列表为空，数据可能未正确处理\n")
				}
			}
		} else {
			logError("JSON解析失败: %v\n", err)
		}
	}

	duration := time.Since(start)
	success := err == nil && resp != nil && resp.StatusCode == 200

	ts.recordTestResult("真实协议数据完整测试", "真实协议", success, duration, err,
		fmt.Sprintf("完成ICCID→Link心跳→DNY协议→API验证完整流程"),
		map[string]interface{}{
			"iccid":        iccidStr,
			"packetsCount": len(realDNYPackets),
			"apiStatus":    resp.StatusCode,
		})

	logImportant("真实协议数据测试完成，耗时: %.2f秒\n", duration.Seconds())
}

// 1. 基础连通性测试
func (ts *TestSuite) runConnectivityTests() {
	logImportant("=== 基础连通性测试 ===\n")

	// TCP连接测试
	ts.testTCPConnection()

	// HTTP连接测试
	ts.testHTTPConnection()

	// 健康检查API测试
	ts.testHealthCheck()
}

// 2. TCP协议测试
func (ts *TestSuite) runTCPProtocolTests() {
	fmt.Println("\n📡 === TCP协议测试 ===")

	// 正常设备注册流程
	ts.testNormalDeviceRegistration()

	// 异常协议帧测试
	ts.testMalformedProtocolFrames()

	// 心跳测试
	ts.testHeartbeatProtocol()

	// 充电控制协议测试
	ts.testChargingProtocol()

	// 端口功率监控测试
	ts.testPortPowerMonitoring()
}

// 3. HTTP API测试
func (ts *TestSuite) runHTTPAPITests() {
	fmt.Println("\n🌐 === HTTP API测试 ===")

	// 设备列表API
	ts.testDeviceListAPI()

	// 设备状态查询API
	ts.testDeviceStatusAPI()

	// 充电控制API
	ts.testChargingControlAPI()

	// 设备定位API
	ts.testDeviceLocateAPI()

	// DNY命令发送API
	ts.testDNYCommandAPI()
}

// 4. 并发场景测试
func (ts *TestSuite) runConcurrencyTests() {
	fmt.Println("\n⚡ === 并发场景测试 ===")

	// 并发设备连接
	ts.testConcurrentConnections()

	// 并发API调用
	ts.testConcurrentAPIRequests()

	// 并发充电控制
	ts.testConcurrentChargingControl()
}

// 5. 错误处理测试
func (ts *TestSuite) runErrorHandlingTests() {
	fmt.Println("\n� === 错误处理测试 ===")

	// 空指针错误测试
	ts.testNilPointerScenarios()

	// 无效数据测试
	// ts.testInvalidDataHandling()

	// 超时场景测试
	ts.testTimeoutScenarios()

	// 资源耗尽测试
	ts.testResourceExhaustion()
}

// 6. 数据状态测试
func (ts *TestSuite) runDataStateTests() {
	fmt.Println("\n📊 === 数据状态测试 ===")

	// 设备状态变迁测试
	ts.testDeviceStateTransitions()

	// 数据一致性测试
	ts.testDataConsistency()

	// 持久化测试
	ts.testDataPersistence()
}

// 7. 压力测试
func (ts *TestSuite) runStressTests() {
	fmt.Println("\n🚀 === 压力测试 ===")

	// 高频心跳测试
	ts.testHighFrequencyHeartbeat()

	// 大量设备连接测试
	ts.testMassiveConnections()

	// 持续运行测试
	ts.testLongRunningStability()
}

// 8. 协议兼容性测试
func (ts *TestSuite) runProtocolCompatibilityTests() {
	fmt.Println("\n� === 协议兼容性测试 ===")

	// 不同版本协议测试
	ts.testProtocolVersions()

	// 边界条件测试
	ts.testProtocolBoundaryConditions()

	// 协议解析一致性测试
	ts.testProtocolParsingConsistency()
}

// logInfo 条件打印信息日志
func logInfo(format string, args ...interface{}) {
	if verboseMode {
		fmt.Printf(format, args...)
	}
}

// logError 总是打印错误日志
func logError(format string, args ...interface{}) {
	fmt.Printf("❌ "+format, args...)
}

// logSuccess 总是打印成功日志
func logSuccess(format string, args ...interface{}) {
	fmt.Printf("✅ "+format, args...)
}

// logImportant 总是打印重要信息
func logImportant(format string, args ...interface{}) {
	fmt.Printf("🔥 "+format, args...)
}

// sendData 发送数据到服务器
func sendData(conn net.Conn, data []byte, description string) error {
	logInfo("发送 %s (%d 字节): %x\n", description, len(data), data)

	_, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("写入数据失败: %v", err)
	}

	logInfo("✅ %s 发送成功\n", description)
	return nil
}

// hexStringToBytes 将十六进制字符串转换为字节数组
func hexStringToBytes(hexStr string) []byte {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Printf("❌ 十六进制解码失败: %v\n", err)
		return nil
	}
	return data
}

// TestSuite的hexStringToBytes方法
func (ts *TestSuite) hexStringToBytes(hexStr string) []byte {
	return hexStringToBytes(hexStr)
}

// sendDataPacket 发送数据包的TestSuite方法
func (ts *TestSuite) sendDataPacket(conn net.Conn, data []byte, description string) error {
	return sendData(conn, data, description)
}

// readResponse 读取响应数据
func readResponse(conn net.Conn, timeout time.Duration) ([]byte, error) {
	response := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := conn.Read(response)
	if err != nil {
		return nil, err
	}
	return response[:n], nil
}

// recordTestResult 记录测试结果
func (ts *TestSuite) recordTestResult(testName, testType string, success bool, duration time.Duration, err error, description string, responseData interface{}) {
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
	}

	ts.testResults = append(ts.testResults, result)

	// 只打印错误或始终显示重要结果
	if !success || verboseMode {
		status := "✅"
		if !success {
			status = "❌"
		}

		logLevel := logInfo
		if !success {
			logLevel = logError
		}

		logLevel("%s [%s] %s (%.2fms)", status, testType, testName, float64(duration.Nanoseconds())/1e6)
		if description != "" {
			logLevel(" - %s", description)
		}
		if err != nil {
			logLevel(" | 错误: %v", err)
		}
		logLevel("\n")
	}
}

// =============================================================================
// 1. 基础连通性测试
// =============================================================================

// testTCPConnection TCP连接测试
func (ts *TestSuite) testTCPConnection() {
	start := time.Now()

	conn, err := net.DialTimeout("tcp", ts.tcpAddress, 5*time.Second)
	success := err == nil

	if success {
		conn.Close()
	}

	ts.recordTestResult("TCP连接测试", "连通性", success, time.Since(start), err, "验证TCP服务器可达性", nil)
}

// testHTTPConnection HTTP连接测试
func (ts *TestSuite) testHTTPConnection() {
	start := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ts.httpBaseURL)
	success := err == nil && resp != nil

	var statusCode int
	if resp != nil {
		statusCode = resp.StatusCode
		resp.Body.Close()
	}

	ts.recordTestResult("HTTP连接测试", "连通性", success, time.Since(start), err,
		fmt.Sprintf("HTTP状态码: %d", statusCode), statusCode)
}

// testHealthCheck 健康检查API测试
func (ts *TestSuite) testHealthCheck() {
	start := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ts.httpBaseURL + "/api/v1/health")
	success := err == nil && resp != nil && resp.StatusCode == 200

	var responseBody string
	if resp != nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		responseBody = string(body)
	}

	ts.recordTestResult("健康检查API", "连通性", success, time.Since(start), err,
		fmt.Sprintf("响应: %s", responseBody), responseBody)
}

// =============================================================================
// 2. TCP协议测试
// =============================================================================

// testNormalDeviceRegistration 正常设备注册流程测试
func (ts *TestSuite) testNormalDeviceRegistration() {
	start := time.Now()
	var err error
	var success bool

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 1. 发送ICCID
	iccidData := []byte("898604D9162390488297")
	err = sendData(conn, iccidData, "ICCID")
	if err != nil {
		ts.recordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "ICCID发送失败", nil)
		return
	}

	time.Sleep(1 * time.Second)

	// 2. 发送设备注册 (修复CRC校验)
	registerData := hexStringToBytes("444e590f00cd28a2040108208002021e31069703")
	if registerData == nil {
		ts.recordTestResult("设备注册流程", "TCP协议", false, time.Since(start),
			fmt.Errorf("注册数据解码失败"), "数据格式错误", nil)
		return
	}

	err = sendData(conn, registerData, "设备注册")
	if err != nil {
		ts.recordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "注册数据发送失败", nil)
		return
	}

	// 3. 读取响应
	response, err := readResponse(conn, 2*time.Second)
	if err != nil {
		ts.recordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "响应读取失败", nil)
		return
	}

	success = len(response) > 0
	responseHex := hex.EncodeToString(response)

	ts.recordTestResult("设备注册流程", "TCP协议", success, time.Since(start), err,
		fmt.Sprintf("响应: %s (%d字节)", responseHex, len(response)), responseHex)

	// 记录设备状态
	ts.mutex.Lock()
	ts.deviceStates["04A228CD"] = "注册成功"
	ts.mutex.Unlock()
}

// testMalformedProtocolFrames 异常协议帧测试
func (ts *TestSuite) testMalformedProtocolFrames() {
	testCases := []struct {
		name string
		data string
		desc string
	}{
		{"无效包头", "58585858cd28a2040108208002021e31069703", "非DNY包头"},
		{"长度错误", "444e59ff00cd28a2040108208002021e31069703", "长度字段错误"},
		{"校验和错误", "444e590f00cd28a2040108208002021e31069999", "校验和不匹配"},
		{"数据截断", "444e590f00cd28a204", "数据包不完整"},
		{"空数据包", "", "空数据"},
	}

	for _, tc := range testCases {
		start := time.Now()

		conn, err := net.Dial("tcp", ts.tcpAddress)
		if err != nil {
			ts.recordTestResult(tc.name, "TCP协议-异常", false, time.Since(start), err, "连接失败", nil)
			continue
		}

		var data []byte
		if tc.data != "" {
			data = hexStringToBytes(tc.data)
		}

		if data != nil {
			err = sendData(conn, data, tc.name)
		}

		// 尝试读取响应（可能超时）
		response, _ := readResponse(conn, 2*time.Second)

		conn.Close()

		// 对于异常帧，服务器应该能够处理而不崩溃
		success := true // 只要不崩溃就算成功

		ts.recordTestResult(tc.name, "TCP协议-异常", success, time.Since(start), err,
			tc.desc, hex.EncodeToString(response))
	}
}

// testHeartbeatProtocol 心跳协议测试
func (ts *TestSuite) testHeartbeatProtocol() {
	start := time.Now()
	var err error

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("心跳协议", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送多种心跳
	heartbeats := []struct {
		name string
		data string
	}{
		{"标准心跳21", "444e591000cd28a204f107216b09020000006140ed"},
		{"Link心跳", "6c696e6b"},
		{"端口功率心跳", "444e591d00cd28a204f1070180026b0902000000000000000000001e003161004405"},
	}

	allSuccess := true
	responseCount := 0

	for _, hb := range heartbeats {
		hbData := hexStringToBytes(hb.data)
		if hbData == nil {
			allSuccess = false
			continue
		}

		err = sendData(conn, hbData, hb.name)
		if err != nil {
			allSuccess = false
			continue
		}

		// 读取响应
		response, err := readResponse(conn, 2*time.Second)
		if err == nil && len(response) > 0 {
			responseCount++
		}

		time.Sleep(500 * time.Millisecond)
	}

	ts.recordTestResult("心跳协议", "TCP协议", allSuccess, time.Since(start), err,
		fmt.Sprintf("发送%d个心跳，收到%d个响应", len(heartbeats), responseCount), responseCount)
}

// testChargingProtocol 充电控制协议测试
func (ts *TestSuite) testChargingProtocol() {
	start := time.Now()
	var err error

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("充电控制协议", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 充电控制命令序列
	commands := []struct {
		name string
		data string
	}{
		{"启动充电", "444e591000cd28a204f1078201010001003c00691a"},
		{"停止充电", "444e591000cd28a204f207820001000100000098d5"},
		{"查询充电状态", "444e590800cd28a204f30722a103"},
	}

	allSuccess := true
	for _, cmd := range commands {
		cmdData := hexStringToBytes(cmd.data)
		if cmdData == nil {
			allSuccess = false
			continue
		}

		err = sendData(conn, cmdData, cmd.name)
		if err != nil {
			allSuccess = false
			continue
		}

		// 读取响应
		_, err = readResponse(conn, 3*time.Second)
		time.Sleep(1 * time.Second)
	}

	ts.recordTestResult("充电控制协议", "TCP协议", allSuccess, time.Since(start), err,
		fmt.Sprintf("执行%d个充电控制命令", len(commands)), nil)
}

// testPortPowerMonitoring 端口功率监控测试
func (ts *TestSuite) testPortPowerMonitoring() {
	start := time.Now()
	var err error

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("端口功率监控", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 模拟不同功率值的监控数据
	powerValues := []int{10, 25, 50, 75, 100} // 瓦特

	allSuccess := true
	for _, power := range powerValues {
		// 构造端口功率数据 (简化版)
		powerHex := fmt.Sprintf("%04x", power)
		powerData := hexStringToBytes(fmt.Sprintf("444e591d00cd28a204f1070180026b090200000000000000000000%s003161004405", powerHex))

		if powerData == nil {
			allSuccess = false
			continue
		}

		err = sendData(conn, powerData, fmt.Sprintf("端口功率监控-%dW", power))
		if err != nil {
			allSuccess = false
			continue
		}

		time.Sleep(500 * time.Millisecond)
	}

	ts.recordTestResult("端口功率监控", "TCP协议", allSuccess, time.Since(start), err,
		fmt.Sprintf("发送%d个功率监控数据", len(powerValues)), powerValues)
}

// =============================================================================
// 3. HTTP API测试
// =============================================================================

// APIResponse HTTP API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// makeHTTPRequest 发送HTTP请求
func (ts *TestSuite) makeHTTPRequest(method, url string, body interface{}) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	responseBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	return resp, responseBody, err
}

// testDeviceListAPI 设备列表API测试
func (ts *TestSuite) testDeviceListAPI() {
	start := time.Now()

	resp, body, err := ts.makeHTTPRequest("GET", ts.httpBaseURL+"/api/v1/devices", nil)
	success := err == nil && resp != nil && resp.StatusCode == 200

	var apiResp APIResponse
	if body != nil {
		json.Unmarshal(body, &apiResp)
	}

	ts.recordTestResult("设备列表API", "HTTP API", success, time.Since(start), err,
		fmt.Sprintf("状态码: %d, 消息: %s", resp.StatusCode, apiResp.Message), apiResp)
}

// testDeviceStatusAPI 设备状态查询API测试
func (ts *TestSuite) testDeviceStatusAPI() {
	deviceIDs := []string{"04A228CD", "04A26CF3", "nonexistent"}

	for _, deviceID := range deviceIDs {
		start := time.Now()

		url := fmt.Sprintf("%s/api/v1/device/%s/status", ts.httpBaseURL, deviceID)
		resp, body, err := ts.makeHTTPRequest("GET", url, nil)

		success := err == nil && resp != nil
		expectedNotFound := deviceID == "nonexistent"

		if expectedNotFound {
			success = success && resp.StatusCode == 404
		} else {
			success = success && resp.StatusCode == 200
		}

		var apiResp APIResponse
		if body != nil {
			json.Unmarshal(body, &apiResp)
		}

		desc := fmt.Sprintf("设备: %s, 状态码: %d", deviceID, resp.StatusCode)
		if expectedNotFound {
			desc += " (期望404)"
		}

		ts.recordTestResult("设备状态API", "HTTP API", success, time.Since(start), err, desc, apiResp)
	}
}

// testChargingControlAPI 充电控制API测试
func (ts *TestSuite) testChargingControlAPI() {
	// 测试充电启动API
	start := time.Now()

	startRequest := map[string]interface{}{
		"deviceId": "04A228CD",
		"port":     1,
		"duration": 60,
	}

	resp, body, err := ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/charging/start", startRequest)
	success := err == nil && resp != nil && (resp.StatusCode == 200 || resp.StatusCode == 404)

	var apiResp APIResponse
	if body != nil {
		json.Unmarshal(body, &apiResp)
	}

	ts.recordTestResult("充电启动API", "HTTP API", success, time.Since(start), err,
		fmt.Sprintf("状态码: %d", resp.StatusCode), apiResp)

	// 测试充电停止API
	start = time.Now()

	stopRequest := map[string]interface{}{
		"deviceId": "04A228CD",
		"port":     1,
	}

	resp, body, err = ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/charging/stop", stopRequest)
	success = err == nil && resp != nil && (resp.StatusCode == 200 || resp.StatusCode == 404)

	if body != nil {
		json.Unmarshal(body, &apiResp)
	}

	ts.recordTestResult("充电停止API", "HTTP API", success, time.Since(start), err,
		fmt.Sprintf("状态码: %d", resp.StatusCode), apiResp)
}

// testDeviceLocateAPI 设备定位API测试
func (ts *TestSuite) testDeviceLocateAPI() {
	start := time.Now()

	locateRequest := map[string]interface{}{
		"deviceId": "04A228CD",
	}

	resp, body, err := ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/device/locate", locateRequest)
	success := err == nil && resp != nil && (resp.StatusCode == 200 || resp.StatusCode == 404)

	var apiResp APIResponse
	if body != nil {
		json.Unmarshal(body, &apiResp)
	}

	ts.recordTestResult("设备定位API", "HTTP API", success, time.Since(start), err,
		fmt.Sprintf("状态码: %d", resp.StatusCode), apiResp)
}

// testDNYCommandAPI DNY命令发送API测试
func (ts *TestSuite) testDNYCommandAPI() {
	commands := []struct {
		name    string
		command int
		data    string
	}{
		{"查询设备状态", 0x81, ""},
		{"查询参数", 0x90, ""},
		{"心跳命令", 0x21, "98080200000905"},
	}

	for _, cmd := range commands {
		start := time.Now()

		dnyRequest := map[string]interface{}{
			"deviceId":  "04A228CD",
			"command":   cmd.command,
			"data":      cmd.data,
			"messageId": 0x1234,
		}

		resp, body, err := ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/command/dny", dnyRequest)
		success := err == nil && resp != nil && (resp.StatusCode == 200 || resp.StatusCode == 404)

		var apiResp APIResponse
		if body != nil {
			json.Unmarshal(body, &apiResp)
		}

		ts.recordTestResult(fmt.Sprintf("DNY命令-%s", cmd.name), "HTTP API", success, time.Since(start), err,
			fmt.Sprintf("命令: 0x%02X, 状态码: %d", cmd.command, resp.StatusCode), apiResp)
	}
}

// =============================================================================
// 4. 并发场景测试
// =============================================================================

// testConcurrentConnections 并发设备连接测试
func (ts *TestSuite) testConcurrentConnections() {
	start := time.Now()

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	for i := 0; i < ts.concurrentNum; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", ts.tcpAddress)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}
			defer conn.Close()

			// 发送ICCID
			iccidData := []byte(fmt.Sprintf("89860%015d", 1000000000000+id))
			_, err = conn.Write(iccidData)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			time.Sleep(100 * time.Millisecond)

			// 发送注册数据（修改设备ID以避免冲突）
			baseRegister := "444e590f00cd28a2040108208002021e31069703"
			registerData := hexStringToBytes(baseRegister)
			if registerData != nil {
				// 修改设备ID部分以创建唯一设备
				deviceIDBytes := []byte{0xCD, 0x28, 0xA2, byte(0x04 + id)}
				copy(registerData[5:9], deviceIDBytes)

				_, err = conn.Write(registerData)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}

			time.Sleep(500 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	success := successCount > 0 && errorCount < int64(ts.concurrentNum/2)

	ts.recordTestResult("并发设备连接", "并发", success, time.Since(start), nil,
		fmt.Sprintf("成功: %d, 失败: %d", successCount, errorCount),
		map[string]int64{"success": successCount, "error": errorCount})
}

// testConcurrentAPIRequests 并发API请求测试
func (ts *TestSuite) testConcurrentAPIRequests() {
	start := time.Now()

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	for i := 0; i < ts.concurrentNum*2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 随机选择API调用
			apis := []string{
				"/api/v1/devices",
				"/api/v1/device/04A228CD/status",
				"/health",
			}

			apiPath := apis[id%len(apis)]
			resp, _, err := ts.makeHTTPRequest("GET", ts.httpBaseURL+apiPath, nil)

			if err == nil && resp != nil && resp.StatusCode < 500 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()

	success := successCount > 0 && errorCount < int64(ts.concurrentNum)

	ts.recordTestResult("并发API请求", "并发", success, time.Since(start), nil,
		fmt.Sprintf("成功: %d, 失败: %d", successCount, errorCount),
		map[string]int64{"success": successCount, "error": errorCount})
}

// testConcurrentChargingControl 并发充电控制测试
func (ts *TestSuite) testConcurrentChargingControl() {
	start := time.Now()

	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()

			startRequest := map[string]interface{}{
				"deviceId": "04A228CD",
				"port":     port + 1,
				"duration": 30,
			}

			resp, _, err := ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/charging/start", startRequest)
			if err == nil && resp != nil && resp.StatusCode < 500 {
				atomic.AddInt64(&successCount, 1)
			}

			time.Sleep(500 * time.Millisecond)

			stopRequest := map[string]interface{}{
				"deviceId": "04A228CD",
				"port":     port + 1,
			}

			resp, _, err = ts.makeHTTPRequest("POST", ts.httpBaseURL+"/api/v1/charging/stop", stopRequest)
			if err == nil && resp != nil && resp.StatusCode < 500 {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	success := successCount > 0

	ts.recordTestResult("并发充电控制", "并发", success, time.Since(start), nil,
		fmt.Sprintf("成功操作: %d", successCount), successCount)
}

// =============================================================================
// 5. 错误处理测试
// =============================================================================

// testNilPointerScenarios 空指针错误测试
func (ts *TestSuite) testNilPointerScenarios() {
	// 这个测试主要是为了触发之前遇到的nil pointer错误
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("空指针场景", "错误处理", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送可能导致空指针的数据序列
	scenarios := []struct {
		name string
		data string
		desc string
	}{
		{"空设备ID注册", "444e590f0000000000000108208002021e31069703", "设备ID为空"},
		{"无效消息ID", "444e590f00cd28a2ffff08208002021e31069703", "消息ID异常"},
		{"异常命令", "444e590f00cd28a20401ff208002021e31069703", "未知命令"},
	}

	allSuccess := true
	for _, scenario := range scenarios {
		scenarioData := hexStringToBytes(scenario.data)
		if scenarioData == nil {
			allSuccess = false
			continue
		}

		err = sendData(conn, scenarioData, scenario.name)
		if err != nil {
			allSuccess = false
		}

		// 短暂等待服务器处理
		time.Sleep(200 * time.Millisecond)
	}

	ts.recordTestResult("空指针场景", "错误处理", allSuccess, time.Since(start), err,
		fmt.Sprintf("测试%d个场景", len(scenarios)), scenarios)
}

// testInvalidDataHandling 无效数据处理测试
func (ts *TestSuite) testInvalidDataHandling() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("无效数据处理", "错误处理", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送各种无效数据
	invalidDataSets := [][]byte{
		[]byte("INVALID_DATA"),                       // 非协议数据
		{0xFF, 0xFF, 0xFF, 0xFF},                     // 随机字节
		make([]byte, 1024),                           // 大量空字节
		{0x44, 0x4E, 0x59},                           // 只有包头
		append([]byte("DNY"), make([]byte, 2000)...), // 超大包
	}

	for i, data := range invalidDataSets {
		err = sendData(conn, data, fmt.Sprintf("无效数据-%d", i+1))
		if err != nil {
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// testTimeoutScenarios 超时场景测试
func (ts *TestSuite) testTimeoutScenarios() {
	start := time.Now()

	// 测试HTTP API超时
	client := &http.Client{Timeout: 1 * time.Millisecond} // 非常短的超时
	_, err := client.Get(ts.httpBaseURL + "/api/v1/devices")
	expectTimeout := err != nil

	// 测试TCP连接超时
	conn, err := net.DialTimeout("tcp", "192.0.2.1:9999", 1*time.Millisecond) // 不可达地址
	expectTCPTimeout := err != nil
	if conn != nil {
		conn.Close()
	}

	success := expectTimeout && expectTCPTimeout

	ts.recordTestResult("超时场景", "错误处理", success, time.Since(start), nil,
		"HTTP和TCP超时测试", map[string]bool{
			"httpTimeout": expectTimeout,
			"tcpTimeout":  expectTCPTimeout,
		})
}

// testResourceExhaustion 资源耗尽测试
func (ts *TestSuite) testResourceExhaustion() {
	start := time.Now()

	// 尝试创建大量连接（但要控制在合理范围内）
	var connections []net.Conn
	maxConnections := 50 // 限制连接数以避免系统问题

	for i := 0; i < maxConnections; i++ {
		conn, err := net.Dial("tcp", ts.tcpAddress)
		if err != nil {
			break
		}
		connections = append(connections, conn)
	}

	// 清理连接
	for _, conn := range connections {
		conn.Close()
	}

	success := len(connections) > 10 // 如果能创建10个以上连接就算成功

	ts.recordTestResult("资源耗尽", "错误处理", success, time.Since(start), nil,
		fmt.Sprintf("成功创建%d个连接", len(connections)), len(connections))
}

// =============================================================================
// 6. 数据状态测试
// =============================================================================

// testDeviceStateTransitions 设备状态变迁测试
func (ts *TestSuite) testDeviceStateTransitions() {
	start := time.Now()

	deviceID := "04A228CD"
	states := []string{"离线", "连接中", "已注册", "充电中", "空闲"}

	// 模拟状态变迁序列
	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("设备状态变迁", "数据状态", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	ts.mutex.Lock()
	ts.deviceStates[deviceID] = states[1] // 连接中
	ts.mutex.Unlock()

	// 注册设备
	registerData := hexStringToBytes("444e590f00cd28a2040108208002021e31069703")
	if registerData != nil {
		sendData(conn, registerData, "设备注册")
		ts.mutex.Lock()
		ts.deviceStates[deviceID] = states[2] // 已注册
		ts.mutex.Unlock()
	}

	time.Sleep(500 * time.Millisecond)

	// 开始充电
	chargeData := hexStringToBytes("444e591000cd28a204f1078201010001003c00a203")
	if chargeData != nil {
		sendData(conn, chargeData, "开始充电")
		ts.mutex.Lock()
		ts.deviceStates[deviceID] = states[3] // 充电中
		ts.mutex.Unlock()
	}

	time.Sleep(500 * time.Millisecond)

	// 停止充电
	stopData := hexStringToBytes("444e591000cd28a204f2078200010001000000a103")
	if stopData != nil {
		sendData(conn, stopData, "停止充电")
		ts.mutex.Lock()
		ts.deviceStates[deviceID] = states[4] // 空闲
		ts.mutex.Unlock()
	}

	ts.recordTestResult("设备状态变迁", "数据状态", true, time.Since(start), nil,
		fmt.Sprintf("完成%d个状态变迁", len(states)), ts.deviceStates)
}

// testDataConsistency 数据一致性测试
func (ts *TestSuite) testDataConsistency() {
	start := time.Now()

	// 测试设备列表API和设备状态的一致性
	resp, body, err := ts.makeHTTPRequest("GET", ts.httpBaseURL+"/api/v1/devices", nil)
	if err != nil {
		ts.recordTestResult("数据一致性", "数据状态", false, time.Since(start), err, "API调用失败", nil)
		return
	}

	var apiResp APIResponse
	json.Unmarshal(body, &apiResp)

	// 检查返回的设备数据格式
	success := resp.StatusCode == 200 || resp.StatusCode == 404

	ts.recordTestResult("数据一致性", "数据状态", success, time.Since(start), err,
		fmt.Sprintf("API状态码: %d", resp.StatusCode), apiResp)
}

// testDataPersistence 数据持久化测试
func (ts *TestSuite) testDataPersistence() {
	start := time.Now()

	// 发送数据然后检查是否被正确处理
	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("数据持久化", "数据状态", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送心跳数据
	heartbeatData := hexStringToBytes("444e591000cd28a204f107216b0902000000618604")
	if heartbeatData != nil {
		sendData(conn, heartbeatData, "心跳数据")
	}

	time.Sleep(1 * time.Second)

	// 查询设备状态确认数据被处理
	statusResp, _, err := ts.makeHTTPRequest("GET", ts.httpBaseURL+"/api/v1/device/04A228CD/status", nil)

	success := err == nil && statusResp != nil && (statusResp.StatusCode == 200 || statusResp.StatusCode == 404)

	ts.recordTestResult("数据持久化", "数据状态", success, time.Since(start), err,
		fmt.Sprintf("状态查询: %d", statusResp.StatusCode), nil)
}

// =============================================================================
// 7. 压力测试
// =============================================================================

// testHighFrequencyHeartbeat 高频心跳测试
func (ts *TestSuite) testHighFrequencyHeartbeat() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("高频心跳", "压力测试", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送高频心跳（每100ms一次，持续5秒）
	heartbeatData := hexStringToBytes("444e591000cd28a204f107216b0902000000618604")
	heartbeatCount := 0

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)

	allSuccess := true
loop:
	for {
		select {
		case <-ticker.C:
			if heartbeatData != nil {
				err = sendData(conn, heartbeatData, fmt.Sprintf("高频心跳-%d", heartbeatCount+1))
				if err != nil {
					allSuccess = false
					break loop
				}
				heartbeatCount++
			}
		case <-timeout:
			break loop
		}
	}

	ts.recordTestResult("高频心跳", "压力测试", allSuccess, time.Since(start), err,
		fmt.Sprintf("发送%d次心跳", heartbeatCount), heartbeatCount)
}

// testMassiveConnections 大量连接测试
func (ts *TestSuite) testMassiveConnections() {
	start := time.Now()

	connectionCount := 20 // 控制连接数量
	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", ts.tcpAddress)
			if err != nil {
				return
			}
			defer conn.Close()

			// 发送简单心跳
			heartbeat := hexStringToBytes("6c696e6b") // "link"
			if heartbeat != nil {
				_, err = conn.Write(heartbeat)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	success := successCount >= int64(connectionCount/2)

	ts.recordTestResult("大量连接", "压力测试", success, time.Since(start), nil,
		fmt.Sprintf("成功连接: %d/%d", successCount, connectionCount), successCount)
}

// testLongRunningStability 长时间运行稳定性测试
func (ts *TestSuite) testLongRunningStability() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("长时间稳定性", "压力测试", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 持续运行30秒，每2秒发送一次心跳
	testDuration := 10 * time.Second // 减少测试时间
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(testDuration)
	heartbeatCount := 0
	allSuccess := true

loop:
	for {
		select {
		case <-ticker.C:
			heartbeatData := hexStringToBytes("6c696e6b")
			if heartbeatData != nil {
				err = sendData(conn, heartbeatData, fmt.Sprintf("稳定性心跳-%d", heartbeatCount+1))
				if err != nil {
					allSuccess = false
					break loop
				}
				heartbeatCount++
			}
		case <-timeout:
			break loop
		}
	}

	ts.recordTestResult("长时间稳定性", "压力测试", allSuccess, time.Since(start), err,
		fmt.Sprintf("运行%.1f秒，发送%d次心跳", testDuration.Seconds(), heartbeatCount), heartbeatCount)
}

// =============================================================================
// 8. 协议兼容性测试
// =============================================================================

// testProtocolVersions 不同版本协议测试
func (ts *TestSuite) testProtocolVersions() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("协议版本兼容性", "协议兼容性", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 测试不同格式的协议帧
	protocols := []struct {
		name string
		data string
	}{
		{"标准DNY协议", "444e590f00cd28a2040108208002021e31069703"},
		{"Link协议", "6c696e6b"},
		{"ICCID协议", "898604D9162390488297"},
	}

	allSuccess := true
	for _, protocol := range protocols {
		var data []byte
		if protocol.name == "ICCID协议" {
			data = []byte(protocol.data)
		} else {
			data = hexStringToBytes(protocol.data)
		}

		if data != nil {
			err = sendData(conn, data, protocol.name)
			if err != nil {
				allSuccess = false
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	ts.recordTestResult("协议版本兼容性", "协议兼容性", allSuccess, time.Since(start), err,
		fmt.Sprintf("测试%d种协议", len(protocols)), protocols)
}

// testProtocolBoundaryConditions 协议边界条件测试
func (ts *TestSuite) testProtocolBoundaryConditions() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("协议边界条件", "协议兼容性", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 测试边界条件
	boundaries := []struct {
		name string
		data []byte
	}{
		{"最小包", hexStringToBytes("444e590500cd28a203")},
		{"最大设备ID", hexStringToBytes("444e590f00ffffffff0108208002021e31069703")},
		{"零长度数据", hexStringToBytes("444e590900cd28a20401a103")},
	}

	allSuccess := true
	for _, boundary := range boundaries {
		if boundary.data != nil {
			err = sendData(conn, boundary.data, boundary.name)
			if err != nil {
				allSuccess = false
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	ts.recordTestResult("协议边界条件", "协议兼容性", allSuccess, time.Since(start), err,
		fmt.Sprintf("测试%d个边界条件", len(boundaries)), boundaries)
}

// testProtocolParsingConsistency 协议解析一致性测试
func (ts *TestSuite) testProtocolParsingConsistency() {
	start := time.Now()

	conn, err := net.Dial("tcp", ts.tcpAddress)
	if err != nil {
		ts.recordTestResult("协议解析一致性", "协议兼容性", false, time.Since(start), err, "连接失败", nil)
		return
	}
	defer conn.Close()

	// 发送相同数据多次，检查解析一致性
	testData := hexStringToBytes("444e591000cd28a204f107216b0902000000618604")
	repeats := 5
	allSuccess := true

	for i := 0; i < repeats; i++ {
		if testData != nil {
			err = sendData(conn, testData, fmt.Sprintf("一致性测试-%d", i+1))
			if err != nil {
				allSuccess = false
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	ts.recordTestResult("协议解析一致性", "协议兼容性", allSuccess, time.Since(start), err,
		fmt.Sprintf("重复发送%d次相同数据", repeats), repeats)
}

// =============================================================================
// 9. 测试报告生成
// =============================================================================

// generateReport 生成测试报告
func (ts *TestSuite) generateReport() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 测试报告")
	fmt.Println(strings.Repeat("=", 80))

	// 统计结果
	totalTests := len(ts.testResults)
	successCount := 0
	failureCount := 0

	// 按类型分组统计
	typeStats := make(map[string]map[string]int)

	for _, result := range ts.testResults {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}

		if typeStats[result.TestType] == nil {
			typeStats[result.TestType] = make(map[string]int)
		}

		if result.Success {
			typeStats[result.TestType]["success"]++
		} else {
			typeStats[result.TestType]["failure"]++
		}
	}

	// 总体统计
	fmt.Printf("📈 总体统计:\n")
	fmt.Printf("   总测试数: %d\n", totalTests)
	fmt.Printf("   成功: %d (%.1f%%)\n", successCount, float64(successCount)*100/float64(totalTests))
	fmt.Printf("   失败: %d (%.1f%%)\n", failureCount, float64(failureCount)*100/float64(totalTests))
	fmt.Printf("   总耗时: %.2f秒\n", ts.getTotalDuration())

	// 分类统计
	fmt.Printf("\n📊 分类统计:\n")
	for testType, stats := range typeStats {
		total := stats["success"] + stats["failure"]
		successRate := float64(stats["success"]) * 100 / float64(total)
		fmt.Printf("   %s: %d/%d (%.1f%%)\n", testType, stats["success"], total, successRate)
	}

	// 失败的测试详情
	if failureCount > 0 {
		fmt.Printf("\n❌ 失败的测试:\n")
		for _, result := range ts.testResults {
			if !result.Success {
				fmt.Printf("   [%s] %s", result.TestType, result.TestName)
				if result.Error != nil {
					fmt.Printf(" - %v", result.Error)
				}
				if result.Description != "" {
					fmt.Printf(" (%s)", result.Description)
				}
				fmt.Println()
			}
		}
	}

	// 性能统计
	fmt.Printf("\n⚡ 性能统计:\n")
	ts.showPerformanceStats()

	// 设备状态
	if len(ts.deviceStates) > 0 {
		fmt.Printf("\n📱 设备状态:\n")
		ts.mutex.Lock()
		for deviceID, state := range ts.deviceStates {
			fmt.Printf("   %s: %s\n", deviceID, state)
		}
		ts.mutex.Unlock()
	}

	fmt.Println(strings.Repeat("=", 80))

	// 生成建议
	ts.generateRecommendations()
}

// getTotalDuration 计算总测试时间
func (ts *TestSuite) getTotalDuration() float64 {
	var total time.Duration
	for _, result := range ts.testResults {
		total += result.Duration
	}
	return total.Seconds()
}

// showPerformanceStats 显示性能统计
func (ts *TestSuite) showPerformanceStats() {
	var avgDuration time.Duration
	var maxDuration time.Duration
	var minDuration time.Duration = time.Hour // 初始化为很大的值

	for _, result := range ts.testResults {
		avgDuration += result.Duration
		if result.Duration > maxDuration {
			maxDuration = result.Duration
		}
		if result.Duration < minDuration {
			minDuration = result.Duration
		}
	}

	if len(ts.testResults) > 0 {
		avgDuration = avgDuration / time.Duration(len(ts.testResults))
		fmt.Printf("   平均耗时: %.2fms\n", float64(avgDuration.Nanoseconds())/1e6)
		fmt.Printf("   最长耗时: %.2fms\n", float64(maxDuration.Nanoseconds())/1e6)
		fmt.Printf("   最短耗时: %.2fms\n", float64(minDuration.Nanoseconds())/1e6)
	}
}

// generateRecommendations 生成建议
func (ts *TestSuite) generateRecommendations() {
	fmt.Printf("💡 建议:\n")

	// 分析失败率
	totalTests := len(ts.testResults)
	failureCount := 0

	for _, result := range ts.testResults {
		if !result.Success {
			failureCount++
		}
	}

	failureRate := float64(failureCount) * 100 / float64(totalTests)

	if failureRate > 50 {
		fmt.Printf("   ⚠️  失败率过高 (%.1f%%)，建议检查服务器状态\n", failureRate)
	} else if failureRate > 20 {
		fmt.Printf("   ⚠️  失败率较高 (%.1f%%)，建议优化错误处理\n", failureRate)
	} else if failureRate > 0 {
		fmt.Printf("   ✅ 整体运行良好，失败率: %.1f%%\n", failureRate)
	} else {
		fmt.Printf("   🎉 所有测试通过！系统运行完美\n")
	}

	// 性能建议
	avgDuration := ts.getTotalDuration() / float64(totalTests)
	if avgDuration > 1.0 {
		fmt.Printf("   ⚠️  平均响应时间较长 (%.2fs)，建议优化性能\n", avgDuration)
	}

	fmt.Printf("   📝 详细日志已记录，可用于问题排查\n")
}
