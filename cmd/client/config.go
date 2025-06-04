package main

import (
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// DeviceConfig 设备配置 - 主机设备配置
type DeviceConfig struct {
	PhysicalID  uint32 // 物理ID
	DeviceType  uint8  // 设备类型
	PortCount   uint8  // 端口数量
	FirmwareVer uint16 // 固件版本
	ICCID       string // ICCID号
	ServerAddr  string // 服务器地址
	// 主机相关配置
	HostType       uint8  // 主机类型
	CommType       uint8  // 通讯模块类型
	RTCType        uint8  // RTC模块类型
	SignalStrength uint8  // 信号强度
	Frequency      uint16 // LORA使用的中心频率
	IMEI           string // 模块的IMEI号
	ModuleVersion  string // 通讯模块的固件版本号
	HasRTC         bool   // 是否有RTC模块
}

// generateUniqueDeviceID 生成唯一的设备ID
// 使用时间戳和随机数确保每次启动都有不同的设备ID基础值
func generateUniqueDeviceID() uint32 {
	// 获取当前时间戳的后3字节作为设备编号
	timestamp := uint32(time.Now().Unix())
	// 取时间戳的低24位，并与设备识别码04组合
	deviceNumber := timestamp & 0x00FFFFFF
	return 0x04000000 | deviceNumber
}

// NewDeviceConfig 创建默认主机设备配置
func NewDeviceConfig() *DeviceConfig {
	return &DeviceConfig{
		PhysicalID:  generateUniqueDeviceID(), // 生成唯一的设备ID
		DeviceType:  0x21,                     // 新款485双模
		PortCount:   2,                        // 双路插座
		FirmwareVer: 200,                      // V2.00
		ICCID:       "89860404D91623904882979",
		ServerAddr:  "localhost:7054",
		// 主机相关默认配置
		HostType:       dny_protocol.HostType485Old, // 旧款485主机
		CommType:       dny_protocol.CommType4G_7S4, // 4G（7S4/G405）
		RTCType:        dny_protocol.RTCTypeSD2068,  // SD2068 RTC模块
		SignalStrength: 20,                          // 信号强度（0-31）
		Frequency:      0,                           // 非LORA设备为0
		IMEI:           "860123456789012",           // 模拟IMEI号
		ModuleVersion:  "4G_MODULE_V1.0.0_20240601", // 模块版本号（24字节）
		HasRTC:         true,                        // 有RTC模块
	}
}

// WithPhysicalID 设置物理ID
func (c *DeviceConfig) WithPhysicalID(id uint32) *DeviceConfig {
	c.PhysicalID = id
	return c
}

// WithDeviceType 设置设备类型
func (c *DeviceConfig) WithDeviceType(deviceType uint8) *DeviceConfig {
	c.DeviceType = deviceType
	return c
}

// WithPortCount 设置端口数量
func (c *DeviceConfig) WithPortCount(portCount uint8) *DeviceConfig {
	c.PortCount = portCount
	return c
}

// WithFirmwareVer 设置固件版本
func (c *DeviceConfig) WithFirmwareVer(version uint16) *DeviceConfig {
	c.FirmwareVer = version
	return c
}

// WithICCID 设置ICCID
func (c *DeviceConfig) WithICCID(iccid string) *DeviceConfig {
	c.ICCID = iccid
	return c
}

// WithServerAddr 设置服务器地址
func (c *DeviceConfig) WithServerAddr(addr string) *DeviceConfig {
	c.ServerAddr = addr
	return c
}
