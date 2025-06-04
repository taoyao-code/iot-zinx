package main

import (
	"time"
)

// DeviceConfig 设备配置
type DeviceConfig struct {
	PhysicalID  uint32 // 物理ID
	DeviceType  uint8  // 设备类型
	PortCount   uint8  // 端口数量
	FirmwareVer uint16 // 固件版本
	ICCID       string // ICCID号
	ServerAddr  string // 服务器地址
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

// NewDeviceConfig 创建默认设备配置
func NewDeviceConfig() *DeviceConfig {
	return &DeviceConfig{
		PhysicalID:  generateUniqueDeviceID(), // 生成唯一的设备ID
		DeviceType:  0x21,                     // 新款485双模
		PortCount:   2,                        // 双路插座
		FirmwareVer: 200,                      // V2.00
		ICCID:       "89860404D91623904882979",
		ServerAddr:  "localhost:7054",
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
