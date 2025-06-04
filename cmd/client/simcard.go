package main

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// SimCard 模拟SIM卡管理结构
type SimCard struct {
	ICCID      string        // SIM卡ICCID号
	DeviceIDs  []uint32      // 管理的设备物理ID列表
	clients    []*TestClient // 关联的设备客户端
	mu         sync.Mutex    // 互斥锁
	serverAddr string        // 服务器地址
	isRunning  bool          // 运行状态
}

// NewSimCard 创建新的SIM卡管理器
func NewSimCard(iccid string, serverAddr string) *SimCard {
	return &SimCard{
		ICCID:      iccid,
		DeviceIDs:  make([]uint32, 0),
		clients:    make([]*TestClient, 0),
		serverAddr: serverAddr,
	}
}

// AddDevice 添加设备物理ID
func (s *SimCard) AddDevice(deviceID uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DeviceIDs = append(s.DeviceIDs, deviceID)
}

// GetDeviceCount 获取设备数量
func (s *SimCard) GetDeviceCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.DeviceIDs)
}

// Start 启动SIM卡管理的所有设备
func (s *SimCard) Start(verbose bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning || len(s.DeviceIDs) == 0 {
		return fmt.Errorf("无法启动SIM卡 %s: 已在运行或没有设备", s.ICCID)
	}

	fmt.Printf("🔌 开始启动SIM卡 %s 下的 %d 个设备\n", s.ICCID, len(s.DeviceIDs))

	// 为每个设备创建客户端
	for i, deviceID := range s.DeviceIDs {
		// 创建设备配置
		config := NewDeviceConfig().
			WithPhysicalID(deviceID).
			WithICCID(s.ICCID). // 所有设备共用同一个ICCID
			WithServerAddr(s.serverAddr)

		// 设置不同的设备类型和端口数量 (为了模拟多样性)
		if i%2 == 0 {
			config.WithDeviceType(0x21).WithPortCount(2) // 双路插座
		} else {
			config.WithDeviceType(0x20).WithPortCount(1) // 单路插座
		}

		// 创建设备客户端
		client := NewTestClient(config)

		// 设置日志级别
		if verbose {
			client.logger.GetLogger().SetLevel(logrus.DebugLevel)
		} else {
			client.logger.GetLogger().SetLevel(logrus.InfoLevel)
		}

		// 保存客户端引用
		s.clients = append(s.clients, client)

		// 打印设备信息
		client.LogInfo()

		// 只有第一个设备发送ICCID (因为SIM卡只有一个)
		if i == 0 {
			// 启动这个设备，并发送ICCID
			if err := client.ConnectAndSendICCID(); err != nil {
				fmt.Printf("❌ SIM卡 %s 的主设备 %08X 连接失败: %s\n", s.ICCID, deviceID, err)
				continue
			}
		} else {
			// 其他设备只需要连接，不发送ICCID
			if err := client.ConnectOnly(); err != nil {
				fmt.Printf("❌ SIM卡 %s 的从设备 %08X 连接失败: %s\n", s.ICCID, deviceID, err)
				continue
			}
		}

		// 发送设备注册包
		if err := client.SendRegister(); err != nil {
			fmt.Printf("❌ 设备 %08X 注册失败: %s\n", deviceID, err)
			continue
		}

		// 启动客户端的心跳和消息处理
		client.StartServices()

		fmt.Printf("✅ 设备 %08X (SIM卡: %s) 启动成功\n", deviceID, s.ICCID)
	}

	s.isRunning = true
	return nil
}

// Stop 停止所有设备
func (s *SimCard) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	fmt.Printf("🛑 停止SIM卡 %s 下的所有设备\n", s.ICCID)

	for _, client := range s.clients {
		client.Stop()
	}

	s.clients = make([]*TestClient, 0)
	s.isRunning = false
}

// RunTestSequence 为所有设备运行测试序列
func (s *SimCard) RunTestSequence() {
	s.mu.Lock()
	clients := make([]*TestClient, len(s.clients))
	copy(clients, s.clients)
	s.mu.Unlock()

	for _, client := range clients {
		go func(c *TestClient) {
			if err := c.RunTestSequence(); err != nil {
				fmt.Printf("❌ 设备 %s 测试序列执行失败: %s\n", c.GetPhysicalIDHex(), err)
			}
		}(client)
	}
}
