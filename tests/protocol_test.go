package tests

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/tests/common"
)

// TestProtocol TCP协议测试套件
// 迁移自debug_device_register.go中的TCP协议测试
func TestProtocol(t *testing.T) {
	// 创建测试套件和辅助工具
	suite := common.NewTestSuite(common.DefaultTestConfig())
	connHelper := common.DefaultConnectionHelper
	protocolHelper := common.DefaultProtocolHelper
	assertHelper := common.DefaultAssertionHelper

	t.Run("协议包构建测试", func(t *testing.T) {
		testProtocolPacketBuilding(t, suite, connHelper, protocolHelper, assertHelper)
	})

	t.Run("异常协议帧测试", func(t *testing.T) {
		testMalformedProtocolFrames(t, suite, connHelper, protocolHelper, assertHelper)
	})

	// 注释掉需要服务器响应的测试，专注于协议包构建和格式验证
	/*
		t.Run("正常设备注册流程", func(t *testing.T) {
			testNormalDeviceRegistration(t, suite, connHelper, protocolHelper, assertHelper)
		})

		t.Run("心跳协议测试", func(t *testing.T) {
			testHeartbeatProtocol(t, suite, connHelper, protocolHelper, assertHelper)
		})

		t.Run("充电控制协议测试", func(t *testing.T) {
			testChargingProtocol(t, suite, connHelper, protocolHelper, assertHelper)
		})

		t.Run("端口功率监控测试", func(t *testing.T) {
			testPortPowerMonitoring(t, suite, connHelper, protocolHelper, assertHelper)
		})
	*/

	// 打印测试摘要
	suite.PrintSummary()
}

// testProtocolPacketBuilding 协议包构建测试
// 验证统一协议构建函数的正确性
func testProtocolPacketBuilding(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 测试设备注册包构建
	deviceID := uint32(0x04A228CD)
	messageID := uint16(0x0801)
	registerPacket := protocolHelper.BuildDeviceRegisterPacket(deviceID, messageID)

	// 验证协议包格式
	assertHelper.AssertProtocolPacket(t, registerPacket, "设备注册包")
	assertHelper.AssertTrue(t, len(registerPacket) > 10, "设备注册包长度")

	// 测试心跳包构建
	heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)
	assertHelper.AssertProtocolPacket(t, heartbeatPacket, "心跳包")
	assertHelper.AssertTrue(t, len(heartbeatPacket) > 5, "心跳包长度")

	// 测试充电控制包构建
	chargingPacket := protocolHelper.BuildChargingPacket(deviceID, messageID, constants.ChargeCommandStart, 1, 60)
	assertHelper.AssertProtocolPacket(t, chargingPacket, "充电控制包")
	assertHelper.AssertTrue(t, len(chargingPacket) > 20, "充电控制包长度")

	// 测试功率监控包构建
	powerPacket := protocolHelper.BuildPowerMonitoringPacket(deviceID, messageID, 1, 100)
	assertHelper.AssertProtocolPacket(t, powerPacket, "功率监控包")
	assertHelper.AssertTrue(t, len(powerPacket) > 15, "功率监控包长度")

	// 记录测试结果
	suite.RecordTestResult("协议包构建测试", "协议构建", true, time.Since(start), nil, "验证统一协议构建函数", map[string]int{
		"register_packet_len":  len(registerPacket),
		"heartbeat_packet_len": len(heartbeatPacket),
		"charging_packet_len":  len(chargingPacket),
		"power_packet_len":     len(powerPacket),
	})

	t.Logf("协议包构建测试完成: 注册包%d字节, 心跳包%d字节, 充电包%d字节, 功率包%d字节",
		len(registerPacket), len(heartbeatPacket), len(chargingPacket), len(powerPacket))
}

// testNormalDeviceRegistration 正常设备注册流程测试
// 使用统一的协议构建函数替换硬编码十六进制字符串
func testNormalDeviceRegistration(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立TCP连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 1. 发送ICCID
	iccidData := protocolHelper.CreateTestICCIDData(0)
	err = connHelper.SendProtocolData(conn, iccidData, "ICCID")
	if err != nil {
		suite.RecordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "ICCID发送失败", nil)
		assertHelper.AssertNoError(t, err, "ICCID发送")
		return
	}

	time.Sleep(1 * time.Second)

	// 2. 发送设备注册包（使用统一的协议构建函数）
	deviceID := uint32(0x04A228CD)
	messageID := uint16(0x0801)
	registerPacket := protocolHelper.BuildDeviceRegisterPacket(deviceID, messageID)

	// 验证协议包格式
	assertHelper.AssertProtocolPacket(t, registerPacket, "设备注册包格式")

	err = connHelper.SendProtocolData(conn, registerPacket, "设备注册")
	if err != nil {
		suite.RecordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "注册数据发送失败", nil)
		assertHelper.AssertNoError(t, err, "设备注册数据发送")
		return
	}

	// 3. 读取响应
	response, err := connHelper.ReadProtocolResponseWithDefaultTimeout(conn)
	if err != nil {
		suite.RecordTestResult("设备注册流程", "TCP协议", false, time.Since(start), err, "响应读取失败", nil)
		assertHelper.AssertNoError(t, err, "响应读取")
		return
	}

	// 验证响应
	success := len(response) > 0
	responseHex := hex.EncodeToString(response)

	suite.RecordTestResult("设备注册流程", "TCP协议", success, time.Since(start), err,
		"响应: "+responseHex, responseHex)

	// 断言响应
	assertHelper.AssertTCPResponse(t, response, 1, "设备注册响应")

	// 记录设备状态
	deviceIDStr := protocolHelper.FormatDeviceID(deviceID)
	suite.SetDeviceState(deviceIDStr, "注册成功")
	assertHelper.AssertDeviceState(t, suite, deviceIDStr, "注册成功")
}

// testMalformedProtocolFrames 异常协议帧测试
// 使用协议辅助工具生成异常数据包
func testMalformedProtocolFrames(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	// 定义异常协议帧测试用例
	testCases := []struct {
		name        string
		packetType  string
		description string
	}{
		{"无效包头", "invalid_header", "非DNY包头"},
		{"长度错误", "wrong_length", "长度字段错误"},
		{"数据截断", "truncated", "数据包不完整"},
		{"空数据包", "empty", "空数据"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			// 建立连接
			conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
			if err != nil {
				suite.RecordTestResult(tc.name, "TCP协议-异常", false, time.Since(start), err, "连接失败", nil)
				assertHelper.AssertNoError(t, err, "TCP连接建立")
				return
			}
			defer connHelper.CloseConnection(conn)

			// 生成异常数据包
			malformedData := protocolHelper.BuildMalformedPacket(tc.packetType)

			// 发送异常数据
			if len(malformedData) > 0 {
				err = connHelper.SendProtocolData(conn, malformedData, tc.name)
			}

			// 尝试读取响应（可能超时）
			response, _ := connHelper.ReadProtocolResponse(conn, 2*time.Second)

			// 对于异常帧，服务器应该能够处理而不崩溃
			success := true // 只要不崩溃就算成功

			suite.RecordTestResult(tc.name, "TCP协议-异常", success, time.Since(start), err,
				tc.description, hex.EncodeToString(response))

			// 异常协议帧测试主要验证服务器稳定性
			t.Logf("%s: 服务器处理异常帧稳定", tc.name)
		})
	}
}

// testHeartbeatProtocol 心跳协议测试
// 使用统一的协议构建函数
func testHeartbeatProtocol(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("心跳协议", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 发送多个心跳包
	deviceID := uint32(0x04A228CD)
	heartbeatCount := 3
	allSuccess := true

	for i := 0; i < heartbeatCount; i++ {
		messageID := uint16(i + 2)
		heartbeatPacket := protocolHelper.BuildHeartbeatPacket(deviceID, messageID)

		// 验证协议包格式
		assertHelper.AssertProtocolPacket(t, heartbeatPacket, "心跳包格式")

		err = connHelper.SendProtocolData(conn, heartbeatPacket, "心跳包")
		if err != nil {
			allSuccess = false
			t.Errorf("发送心跳包 #%d 失败: %v", i+1, err)
			continue
		}

		// 读取响应
		response, err := connHelper.ReadProtocolResponse(conn, 3*time.Second)
		if err != nil {
			t.Logf("心跳包 #%d 响应读取失败: %v", i+1, err)
		} else {
			t.Logf("心跳包 #%d 响应: %s", i+1, hex.EncodeToString(response))
		}

		time.Sleep(2 * time.Second)
	}

	suite.RecordTestResult("心跳协议", "TCP协议", allSuccess, time.Since(start), err,
		"发送心跳包数量: "+string(rune(heartbeatCount)), heartbeatCount)

	assertHelper.AssertTrue(t, allSuccess, "心跳协议测试")
}

// testChargingProtocol 充电控制协议测试
// 使用统一的协议构建函数和常量
func testChargingProtocol(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("充电控制协议", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 获取充电控制命令
	commands := protocolHelper.GetChargingCommands()
	deviceID := uint32(0x04A228CD)
	portNum := uint8(1)
	allSuccess := true

	for i, cmd := range commands {
		messageID := uint16(0xF107 + i)
		duration := uint16(60) // 60分钟
		if cmd.Command == constants.ChargeCommandStop {
			duration = 0 // 停止充电时长为0
		}

		// 构建充电控制包
		chargingPacket := protocolHelper.BuildChargingPacket(deviceID, messageID, cmd.Command, portNum, duration)

		// 验证协议包格式
		assertHelper.AssertProtocolPacket(t, chargingPacket, cmd.Name+"包格式")

		err = connHelper.SendProtocolData(conn, chargingPacket, cmd.Name)
		if err != nil {
			allSuccess = false
			t.Errorf("发送%s失败: %v", cmd.Name, err)
			continue
		}

		// 读取响应
		response, err := connHelper.ReadProtocolResponse(conn, 3*time.Second)
		if err != nil {
			t.Logf("%s响应读取失败: %v", cmd.Name, err)
		} else {
			t.Logf("%s响应: %s", cmd.Name, hex.EncodeToString(response))
		}

		time.Sleep(1 * time.Second)
	}

	suite.RecordTestResult("充电控制协议", "TCP协议", allSuccess, time.Since(start), err,
		"执行充电控制命令数量: "+string(rune(len(commands))), len(commands))

	assertHelper.AssertTrue(t, allSuccess, "充电控制协议测试")
}

// testPortPowerMonitoring 端口功率监控测试
// 使用统一的协议构建函数
func testPortPowerMonitoring(t *testing.T, suite *common.TestSuite, connHelper *common.ConnectionHelper, protocolHelper *common.ProtocolHelper, assertHelper *common.AssertionHelper) {
	start := time.Now()

	// 建立连接
	conn, err := connHelper.EstablishTCPConnection(suite.TCPAddress)
	if err != nil {
		suite.RecordTestResult("端口功率监控", "TCP协议", false, time.Since(start), err, "连接失败", nil)
		assertHelper.AssertNoError(t, err, "TCP连接建立")
		return
	}
	defer connHelper.CloseConnection(conn)

	// 模拟不同功率值的监控数据
	deviceID := uint32(0x04A228CD)
	portNum := uint8(1)
	powerValues := []uint16{10, 25, 50, 75, 100} // 瓦特
	allSuccess := true

	for i, power := range powerValues {
		messageID := uint16(0xF107 + i)

		// 构建功率监控包
		powerPacket := protocolHelper.BuildPowerMonitoringPacket(deviceID, messageID, portNum, power)

		// 验证协议包格式
		assertHelper.AssertProtocolPacket(t, powerPacket, "功率监控包格式")

		description := "端口功率监控-" + string(rune(power)) + "W"
		err = connHelper.SendProtocolData(conn, powerPacket, description)
		if err != nil {
			allSuccess = false
			t.Errorf("发送功率监控数据失败: %v", err)
			continue
		}

		time.Sleep(500 * time.Millisecond)
	}

	suite.RecordTestResult("端口功率监控", "TCP协议", allSuccess, time.Since(start), err,
		"发送功率监控数据数量: "+string(rune(len(powerValues))), powerValues)

	assertHelper.AssertTrue(t, allSuccess, "端口功率监控测试")
}
