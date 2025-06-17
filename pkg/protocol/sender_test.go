package protocol

import (
	"testing"
)

// TestNeedConfirmation 测试命令确认机制
func TestNeedConfirmation(t *testing.T) {
	// 测试不需要确认的指令
	noConfirmationCommands := []uint8{
		0x22, // 获取服务器时间
		0x81, // 查询设备联网状态
		0x06, // 端口充电时功率心跳包
		0x41, // 充电柜专有心跳包
		0x42, // 报警推送指令
		0x43, // 充电完成通知
		0x44, // 端口推送指令
		0x05, // 设备主动请求升级
		0x09, // 分机测试模式
		0x0A, // 分机设置主机模块地址
		0x90, 0x91, 0x92, 0x93, // 查询参数指令
		0x01, 0x11, 0x21, // 各种心跳包
	}

	for _, cmd := range noConfirmationCommands {
		if NeedConfirmation(cmd) {
			t.Errorf("命令 0x%02X 应该不需要确认，但函数返回需要确认", cmd)
		}
	}

	// 测试需要确认的指令
	confirmationCommands := []uint8{
		0x82, // 开始/停止充电操作
		0x83, // 设置运行参数1.1
		0x84, // 设置运行参数1.2
		0x85, // 设置最大充电时长、过载功率
		0x86, // 设置用户卡参数
		0x87, // 复位重启设备
		0x88, // 存储器清零
		0x8A, // 修改充电时长/电量
		0x8D, // 设置设备的工作模式
		0x8E, // 修改二维码地址
		0x8F, // 设置设备TC刷卡模式
		0xE0, // 设备固件升级
		0xF8, // 设备固件升级(老版本)
	}

	for _, cmd := range confirmationCommands {
		if !NeedConfirmation(cmd) {
			t.Errorf("命令 0x%02X 应该需要确认，但函数返回不需要确认", cmd)
		}
	}
}

// TestNeedConfirmation_SpecialCases 测试特殊情况
func TestNeedConfirmation_SpecialCases(t *testing.T) {
	// 测试0x22命令（这是我们修复的核心问题）
	if NeedConfirmation(0x22) {
		t.Error("0x22命令（获取服务器时间）不应该需要确认，根据协议文档设备收到应答后停止发送")
	}

	// 测试心跳类命令
	heartbeatCommands := []uint8{0x01, 0x11, 0x21}
	for _, cmd := range heartbeatCommands {
		if NeedConfirmation(cmd) {
			t.Errorf("心跳命令 0x%02X 不应该需要确认", cmd)
		}
	}

	// 测试查询类命令
	queryCommands := []uint8{0x81, 0x90, 0x91, 0x92, 0x93}
	for _, cmd := range queryCommands {
		if NeedConfirmation(cmd) {
			t.Errorf("查询命令 0x%02X 不应该需要确认", cmd)
		}
	}
}

// BenchmarkNeedConfirmation 性能测试
func BenchmarkNeedConfirmation(b *testing.B) {
	commands := []uint8{0x22, 0x81, 0x82, 0x06, 0x41, 0x90}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmd := range commands {
			NeedConfirmation(cmd)
		}
	}
}
