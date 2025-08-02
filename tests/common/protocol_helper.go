package common

import (
	"encoding/hex"
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// ProtocolHelper 协议测试辅助工具
type ProtocolHelper struct{}

// NewProtocolHelper 创建协议辅助工具实例
func NewProtocolHelper() *ProtocolHelper {
	return &ProtocolHelper{}
}

// BuildDeviceRegisterPacket 构建设备注册数据包
// 使用统一的协议构建函数替换硬编码十六进制字符串
func (ph *ProtocolHelper) BuildDeviceRegisterPacket(deviceID uint32, messageID uint16) []byte {
	// 构建设备注册数据
	registerData := make([]byte, 8)
	
	// 设备类型 (1字节)
	registerData[0] = 0x04
	
	// 版本号 (1字节)  
	registerData[1] = 0x01
	
	// 心跳周期 (2字节，小端序)
	registerData[2] = 0x08
	registerData[3] = 0x20
	
	// 端口数量 (1字节)
	registerData[4] = 0x80
	
	// 工作模式 (1字节)
	registerData[5] = 0x02
	
	// 预留字段 (2字节)
	registerData[6] = 0x02
	registerData[7] = 0x1e
	
	// 使用统一的协议包构建函数
	return dny_protocol.BuildDNYPacket(deviceID, messageID, 0x20, registerData)
}

// BuildHeartbeatPacket 构建心跳数据包
func (ph *ProtocolHelper) BuildHeartbeatPacket(deviceID uint32, messageID uint16) []byte {
	// 简单的心跳数据
	heartbeatData := []byte{0x01}
	
	// 使用统一的协议包构建函数
	return dny_protocol.BuildDNYPacket(deviceID, messageID, 0x01, heartbeatData)
}

// BuildChargingPacket 构建充电控制数据包
func (ph *ProtocolHelper) BuildChargingPacket(deviceID uint32, messageID uint16, command uint8, portNum uint8, duration uint16) []byte {
	// 构建充电控制数据
	chargingData := make([]byte, 37) // 0x82命令的数据长度
	
	// 端口号 (1字节)
	chargingData[0] = portNum
	
	// 充电命令 (1字节)
	chargingData[1] = command
	
	// 充电时长 (2字节，小端序)
	chargingData[2] = byte(duration & 0xFF)
	chargingData[3] = byte((duration >> 8) & 0xFF)
	
	// 订单号 (4字节) - 使用简单的递增值
	chargingData[4] = 0x01
	chargingData[5] = 0x00
	chargingData[6] = 0x00
	chargingData[7] = 0x00
	
	// 其余字段填充默认值
	for i := 8; i < len(chargingData); i++ {
		chargingData[i] = 0x00
	}
	
	// 使用统一的协议包构建函数
	return dny_protocol.BuildDNYPacket(deviceID, messageID, 0x82, chargingData)
}

// BuildPowerMonitoringPacket 构建功率监控数据包
func (ph *ProtocolHelper) BuildPowerMonitoringPacket(deviceID uint32, messageID uint16, portNum uint8, power uint16) []byte {
	// 构建功率监控数据
	powerData := make([]byte, 29) // 功率监控数据长度
	
	// 端口号 (1字节)
	powerData[0] = portNum
	
	// 功率值 (2字节，小端序)
	powerData[1] = byte(power & 0xFF)
	powerData[2] = byte((power >> 8) & 0xFF)
	
	// 电压值 (2字节) - 模拟220V
	voltage := uint16(220)
	powerData[3] = byte(voltage & 0xFF)
	powerData[4] = byte((voltage >> 8) & 0xFF)
	
	// 电流值 (2字节) - 根据功率计算
	current := uint16(0)
	if voltage > 0 {
		current = power * 1000 / voltage // mA
	}
	powerData[5] = byte(current & 0xFF)
	powerData[6] = byte((current >> 8) & 0xFF)
	
	// 其余字段填充默认值
	for i := 7; i < len(powerData); i++ {
		powerData[i] = 0x00
	}
	
	// 使用统一的协议包构建函数
	return dny_protocol.BuildDNYPacket(deviceID, messageID, 0x06, powerData)
}

// BuildMalformedPacket 构建异常协议包用于测试
func (ph *ProtocolHelper) BuildMalformedPacket(packetType string) []byte {
	switch packetType {
	case "invalid_header":
		// 无效包头
		return []byte{0x58, 0x58, 0x58, 0x58, 0xcd, 0x28, 0xa2, 0x04}
		
	case "wrong_length":
		// 长度字段错误
		packet := ph.BuildDeviceRegisterPacket(0x04A228CD, 0x0801)
		if len(packet) >= 5 {
			packet[3] = 0xFF // 修改长度字段
			packet[4] = 0x00
		}
		return packet
		
	case "truncated":
		// 数据包截断
		packet := ph.BuildDeviceRegisterPacket(0x04A228CD, 0x0801)
		if len(packet) > 10 {
			return packet[:10] // 截断数据包
		}
		return packet
		
	case "empty":
		// 空数据包
		return []byte{}
		
	default:
		// 默认返回正常包
		return ph.BuildDeviceRegisterPacket(0x04A228CD, 0x0801)
	}
}

// ParseHexString 解析十六进制字符串为字节数组
// 兼容原有的hexStringToBytes函数
func (ph *ProtocolHelper) ParseHexString(hexStr string) []byte {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return data
}

// FormatDeviceID 格式化设备ID
// 使用统一的设备ID格式化函数
func (ph *ProtocolHelper) FormatDeviceID(deviceID uint32) string {
	return utils.FormatPhysicalID(deviceID)
}

// GetTestDeviceIDs 获取测试用的设备ID列表
func (ph *ProtocolHelper) GetTestDeviceIDs() []uint32 {
	return []uint32{
		0x04A228CD, // 主要测试设备
		0x04A26CF3, // 从设备
		0x04A22001, // 测试设备1
		0x04A22002, // 测试设备2
		0x04A22003, // 测试设备3
	}
}

// GetChargingCommands 获取充电命令列表
func (ph *ProtocolHelper) GetChargingCommands() []struct {
	Name    string
	Command uint8
	Desc    string
} {
	return []struct {
		Name    string
		Command uint8
		Desc    string
	}{
		{"启动充电", constants.ChargeCommandStart, "开始充电"},
		{"停止充电", constants.ChargeCommandStop, "停止充电"},
		{"查询状态", constants.ChargeCommandQuery, "查询充电状态"},
	}
}

// ValidateProtocolPacket 验证协议包格式
func (ph *ProtocolHelper) ValidateProtocolPacket(packet []byte) error {
	if len(packet) < 3 {
		return fmt.Errorf("数据包太短，至少需要3字节")
	}
	
	// 检查DNY包头
	if string(packet[:3]) != "DNY" {
		return fmt.Errorf("无效的协议包头，期望'DNY'，实际'%s'", string(packet[:3]))
	}
	
	if len(packet) < 5 {
		return fmt.Errorf("数据包缺少长度字段")
	}
	
	// 检查长度字段
	expectedLen := int(packet[3]) | (int(packet[4]) << 8)
	actualLen := len(packet) - 5 // 减去包头和长度字段
	
	if actualLen != expectedLen {
		return fmt.Errorf("数据包长度不匹配，期望%d字节，实际%d字节", expectedLen, actualLen)
	}
	
	return nil
}

// CreateTestICCIDData 创建测试用的ICCID数据
func (ph *ProtocolHelper) CreateTestICCIDData(index int) []byte {
	// 生成测试用的ICCID
	iccid := fmt.Sprintf("89860%015d", 1000000000000+index)
	return []byte(iccid)
}

// GetProtocolCommandName 获取协议命令名称
func (ph *ProtocolHelper) GetProtocolCommandName(command uint8) string {
	commandNames := map[uint8]string{
		0x01: "心跳",
		0x06: "功率心跳", 
		0x20: "设备注册",
		0x21: "简化心跳",
		0x22: "获取服务器时间",
		0x23: "结算数据",
		0x82: "充电控制",
		0xF1: "刷卡请求",
	}
	
	if name, exists := commandNames[command]; exists {
		return name
	}
	return fmt.Sprintf("未知命令(0x%02X)", command)
}

// 全局协议辅助工具实例
var DefaultProtocolHelper = NewProtocolHelper()
