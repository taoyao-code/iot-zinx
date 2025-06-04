package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// TestClient 测试客户端结构体
type TestClient struct {
	conn      net.Conn
	config    *DeviceConfig
	messageID uint16 // 消息ID计数器
	isRunning bool   // 运行状态
	logger    *logger.ImprovedLogger
	mu        sync.Mutex
	stopChan  chan struct{}
	isMaster  bool // 是否为主设备（负责发送ICCID）
}

// NewTestClient 创建新的测试客户端
func NewTestClient(config *DeviceConfig) *TestClient {
	// 如果没有提供配置，使用默认配置
	if config == nil {
		config = NewDeviceConfig()
	}

	// 创建客户端
	client := &TestClient{
		config:    config,
		messageID: 1, // 消息ID从1开始
		isRunning: false,
		stopChan:  make(chan struct{}),
	}

	// 初始化日志系统
	client.initLogger()

	return client
}

// initLogger 初始化日志系统
func (c *TestClient) initLogger() {
	// 创建改进的日志记录器
	improvedLogger := logger.NewImprovedLogger()
	improvedLogger.GetLogger().SetLevel(logrus.InfoLevel)
	improvedLogger.GetLogger().SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		ForceColors:     true,
	})
	c.logger = improvedLogger
}

// Connect 连接到服务器
func (c *TestClient) Connect() error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"address": c.config.ServerAddr,
	}).Info("🔗 开始连接服务器...")

	conn, err := net.Dial("tcp", c.config.ServerAddr)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 连接服务器失败")
		return err
	}

	c.conn = conn
	c.logger.GetLogger().WithFields(logrus.Fields{
		"localAddr":  conn.LocalAddr().String(),
		"remoteAddr": conn.RemoteAddr().String(),
		"physicalID": fmt.Sprintf("0x%08X", c.config.PhysicalID),
	}).Info("✅ 连接服务器成功")

	return nil
}

// SendICCID 发送ICCID号码
func (c *TestClient) SendICCID() error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"iccid": c.config.ICCID,
	}).Info("📤 发送ICCID...")

	_, err := c.conn.Write([]byte(c.config.ICCID))
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送ICCID失败")
		return err
	}

	c.logger.GetLogger().Info("✅ ICCID发送成功")
	return nil
}

// ConnectAndSendICCID 连接并发送ICCID（主设备使用）
func (c *TestClient) ConnectAndSendICCID() error {
	c.isMaster = true

	// 连接服务器
	if err := c.Connect(); err != nil {
		return err
	}

	// 发送ICCID
	if err := c.SendICCID(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"iccid":    c.config.ICCID,
		"deviceId": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"isMaster": true,
	}).Info("✅ 主设备连接并发送ICCID成功")

	return nil
}

// ConnectOnly 仅连接服务器（从设备使用）
func (c *TestClient) ConnectOnly() error {
	c.isMaster = false

	// 连接服务器
	if err := c.Connect(); err != nil {
		return err
	}

	c.logger.GetLogger().WithFields(logrus.Fields{
		"iccid":    c.config.ICCID,
		"deviceId": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"isMaster": false,
	}).Info("✅ 从设备连接成功")

	return nil
}

// StartHeartbeat 启动主机协议心跳协程
func (c *TestClient) StartHeartbeat() {
	c.logger.GetLogger().Info("💓 启动主机协议心跳协程...")

	// 启动"link"心跳协程（每30秒）
	go c.startLinkHeartbeat()

	// 启动主机状态心跳协程（每30分钟）
	go c.startMainHeartbeat()
}

// startLinkHeartbeat 启动"link"心跳协程（每30秒当没有数据时）
func (c *TestClient) startLinkHeartbeat() {
	c.logger.GetLogger().Info("💓 启动 'link' 心跳协程，间隔30秒...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.isRunning {
				// 发送简单的"link"字符串作为心跳
				if err := c.SendLinkHeartbeat(); err != nil {
					c.logger.GetLogger().WithError(err).Error("❌ link心跳发送失败")
				}
			}
		case <-c.stopChan:
			c.logger.GetLogger().Info("🛑 link心跳协程已停止")
			return
		}
	}
}

// startMainHeartbeat 启动主机状态心跳协程（每30分钟）
func (c *TestClient) startMainHeartbeat() {
	c.logger.GetLogger().Info("💓 启动主机状态心跳协程，间隔30分钟...")

	// 首次立即发送一次心跳包
	time.Sleep(5 * time.Second) // 等待连接稳定
	if c.isRunning {
		if err := c.SendMainHeartbeat(); err != nil {
			c.logger.GetLogger().WithError(err).Error("❌ 初始主机心跳发送失败")
		}
		time.Sleep(2 * time.Second)
		if err := c.SendMainStatusReport(); err != nil {
			c.logger.GetLogger().WithError(err).Error("❌ 初始主机状态包发送失败")
		}
	}

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.isRunning {
				// 发送主机状态心跳包
				if err := c.SendMainHeartbeat(); err != nil {
					c.logger.GetLogger().WithError(err).Error("❌ 主机心跳发送失败")
				}

				// 等待2秒后发送状态包
				time.Sleep(2 * time.Second)
				if err := c.SendMainStatusReport(); err != nil {
					c.logger.GetLogger().WithError(err).Error("❌ 主机状态包发送失败")
				}
			}
		case <-c.stopChan:
			c.logger.GetLogger().Info("🛑 主机心跳协程已停止")
			return
		}
	}
}

// Start 启动主机协议客户端
func (c *TestClient) Start() error {
	// 连接服务器
	if err := c.Connect(); err != nil {
		return err
	}

	// 设置运行状态
	c.isRunning = true

	// 启动消息处理协程
	go c.HandleServerMessages()

	// 1. 发送ICCID（主机协议第一步）
	if err := c.SendICCID(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	// 2. 请求服务器时间（主机协议第二步）
	if err := c.SendGetServerTime(); err != nil {
		c.logger.GetLogger().WithError(err).Warn("⚠️ 获取服务器时间失败，继续执行...")
	}
	time.Sleep(2 * time.Second)

	// 3. 发送设备注册包（兼容性）
	if err := c.SendRegister(); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	// 4. 启动主机协议心跳机制
	c.StartHeartbeat()

	c.logger.GetLogger().Info("✅ 主机协议客户端启动完成")
	return nil
}

// Stop 停止客户端
func (c *TestClient) Stop() {
	c.logger.GetLogger().Info("🛑 停止客户端...")

	c.isRunning = false
	close(c.stopChan)

	if c.conn != nil {
		c.conn.Close()
	}

	c.logger.GetLogger().Info("✅ 客户端已停止")
}

// getNextMessageID 获取下一个消息ID
func (c *TestClient) getNextMessageID() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.messageID
	c.messageID++
	if c.messageID == 0 {
		c.messageID = 1 // 避免使用0作为消息ID
	}
	return id
}

// LogInfo 记录设备信息
func (c *TestClient) LogInfo() {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":  fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"deviceType":  fmt.Sprintf("0x%02X (新款485双模)", c.config.DeviceType),
		"portCount":   c.config.PortCount,
		"firmwareVer": fmt.Sprintf("V%d.%02d", c.config.FirmwareVer/100, c.config.FirmwareVer%100),
		"iccid":       c.config.ICCID,
		"serverAddr":  c.config.ServerAddr,
	}).Info("🔧 客户端配置")
}

// RunTestSequence 运行测试序列
func (c *TestClient) RunTestSequence() error {
	c.logger.GetLogger().Info("🎯 开始执行测试序列...")

	// 等待一下让连接稳定
	time.Sleep(2 * time.Second)

	// 测试刷卡操作
	c.logger.GetLogger().Info("🧪 测试1: 发送刷卡操作...")
	if err := c.SendSwipeCard(0xDD058D7A, 1); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	// 测试结算信息
	c.logger.GetLogger().Info("🧪 测试2: 发送结算信息...")
	if err := c.SendSettlement(1, 1800, 1000, 150); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	// 发送一个额外的心跳包
	c.logger.GetLogger().Info("🧪 测试3: 发送额外心跳包...")
	if err := c.SendHeartbeat(); err != nil {
		return err
	}

	c.logger.GetLogger().Info("✅ 测试序列执行完成")
	return nil
}

// 返回设备配置
func (c *TestClient) GetConfig() *DeviceConfig {
	return c.config
}

// 设置设备配置
func (c *TestClient) SetConfig(config *DeviceConfig) {
	c.config = config
}

// GetPhysicalID 返回设备物理ID的十六进制字符串表示
func (c *TestClient) GetPhysicalIDHex() string {
	return fmt.Sprintf("%08X", c.config.PhysicalID)
}

// StartServices 启动客户端的各项服务
func (c *TestClient) StartServices() {
	// 设置运行状态
	c.isRunning = true

	// 启动消息处理协程
	go c.HandleServerMessages()

	// 启动心跳
	c.StartHeartbeat()

	c.logger.GetLogger().WithFields(logrus.Fields{
		"deviceId": fmt.Sprintf("0x%08X", c.config.PhysicalID),
		"isMaster": c.isMaster,
	}).Info("📡 设备服务已启动")
}
