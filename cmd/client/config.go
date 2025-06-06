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
	VirtualID   uint8  // 虚拟ID（用于区分同一ICCID下的不同设备）
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
	IsMaster       bool   // 是否是主机设备
}

// 设备类型判断常量
const (
	DeviceTypeMaster = 0x09 // 主机设备类型前缀（09开头）
	DeviceTypeSlave  = 0x04 // 分机设备类型前缀（04开头）
)

// generateUniqueDeviceID 生成唯一的设备ID
// 使用时间戳和随机数确保每次启动都有不同的设备ID基础值
func generateUniqueDeviceID() uint32 {
	// 获取当前时间戳的后3字节作为设备编号
	timestamp := uint32(time.Now().Unix())
	// 取时间戳的低24位，并与设备识别码04组合
	deviceNumber := timestamp & 0x00FFFFFF
	return 0x04000000 | deviceNumber
}

// generateUniqueMasterID 生成唯一的主机ID
func generateUniqueMasterID() uint32 {
	// 获取当前时间戳的后3字节作为设备编号
	timestamp := uint32(time.Now().Unix())
	// 取时间戳的低24位，并与主机识别码09组合
	deviceNumber := timestamp & 0x00FFFFFF
	return 0x09000000 | deviceNumber
}

// IsMasterDevice 判断设备ID是否为主机
func IsMasterDevice(deviceID uint32) bool {
	// 获取设备ID的高字节，判断是否为主机类型
	return (deviceID >> 24) == DeviceTypeMaster
}

// NewDeviceConfig 创建默认主机设备配置 - 基于线上真实数据
func NewDeviceConfig() *DeviceConfig {
	return &DeviceConfig{
		PhysicalID:  generateUniqueDeviceID(), // 生成唯一的设备ID
		DeviceType:  0x31,                     // 设备类型49（线上数据）
		PortCount:   2,                        // 双路插座
		FirmwareVer: 640,                      // 固件版本640（线上数据）
		VirtualID:   0x1e,                     // 虚拟ID 30（线上数据）
		ICCID:       "898604D9162390488297",   // 真实ICCID（线上数据）
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
		IsMaster:       false,                       // 默认为分机设备
	}
}

// NewMasterDeviceConfig 创建默认主机设备配置
func NewMasterDeviceConfig() *DeviceConfig {
	config := NewDeviceConfig()
	config.PhysicalID = generateUniqueMasterID()
	config.IsMaster = true
	return config
}

// NewDeviceConfigWithPhysicalID 创建指定物理ID的设备配置
func NewDeviceConfigWithPhysicalID(physicalID uint32, virtualID uint8) *DeviceConfig {
	config := NewDeviceConfig()
	config.PhysicalID = physicalID
	config.VirtualID = virtualID
	return config
}

// CreateMultipleDevicesConfig 创建多个设备配置（模拟线上多设备场景）
func CreateMultipleDevicesConfig() []*DeviceConfig {
	// 创建一个主机和一个分机，共享同一个ICCID
	devices := []*DeviceConfig{
		NewDeviceConfigWithPhysicalID(0x09A228CD, 0x1e), // 主机设备 (09开头)
		NewDeviceConfigWithPhysicalID(0x04A26CF3, 0x1f), // 分机设备 (04开头)
	}

	// 标记设备类型
	devices[0].IsMaster = true
	devices[1].IsMaster = false

	// 设置相同的ICCID
	iccid := "898604D9162390488297"
	for _, device := range devices {
		device.ICCID = iccid
	}

	return devices
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
