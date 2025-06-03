package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// TestClient 测试客户端结构体
type TestClient struct {
	conn        net.Conn
	physicalID  uint32 // 物理ID
	deviceType  uint8  // 设备类型
	portCount   uint8  // 端口数量
	firmwareVer uint16 // 固件版本
	messageID   uint16 // 消息ID计数器
	isRunning   bool   // 运行状态
	logger      *logger.ImprovedLogger
	mu          sync.Mutex
}

// NewTestClient 创建新的测试客户端
func NewTestClient() *TestClient {
	// 根据协议文档设置设备参数
	// 设备编号：13544000，识别码：04（双路插座）
	// 物理ID编码：小端模式，04ceaa40 (大端) -> 40aace04 (小端)
	physicalID := uint32(0x04ceaa40) // 设备识别码04 + 设备编号13544000

	return &TestClient{
		physicalID:  physicalID,
		deviceType:  0x21, // 新款485双模
		portCount:   2,    // 双路插座
		firmwareVer: 200,  // V2.00
		messageID:   1,    // 消息ID从1开始
		isRunning:   false,
	}
}

// Connect 连接到服务器
func (c *TestClient) Connect(address string) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"address": address,
	}).Info("🔗 开始连接服务器...")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 连接服务器失败")
		return err
	}

	c.conn = conn
	c.logger.GetLogger().WithFields(logrus.Fields{
		"localAddr":  conn.LocalAddr().String(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Info("✅ 连接服务器成功")

	return nil
}

// SendICCID 发送ICCID号码
func (c *TestClient) SendICCID() error {
	iccid := "89860404D91623904882979"
	c.logger.GetLogger().WithFields(logrus.Fields{
		"iccid": iccid,
	}).Info("📤 发送ICCID...")

	_, err := c.conn.Write([]byte(iccid))
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送ICCID失败")
		return err
	}

	c.logger.GetLogger().Info("✅ ICCID发送成功")
	return nil
}

// SendRegister 发送设备注册包（20指令）
func (c *TestClient) SendRegister() error {
	c.logger.GetLogger().Info("📤 发送设备注册包（0x20指令）...")

	// 构建注册包数据
	data := make([]byte, 8)

	// 固件版本（2字节，小端序）
	binary.LittleEndian.PutUint16(data[0:2], c.firmwareVer)

	// 端口数量（1字节）
	data[2] = c.portCount

	// 虚拟ID（1字节）- 不需组网设备默认为00
	data[3] = 0x00

	// 设备类型（1字节）
	data[4] = c.deviceType

	// 工作模式（1字节）- 第0位：0=联网，其他位保留
	data[5] = 0x00

	// 电源板版本号（2字节）- 无电源板为0
	binary.LittleEndian.PutUint16(data[6:8], 0)

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.physicalID, c.getNextMessageID(), dny_protocol.CmdDeviceRegister, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":  fmt.Sprintf("0x%08X", c.physicalID),
		"deviceType":  fmt.Sprintf("0x%02X", c.deviceType),
		"firmwareVer": c.firmwareVer,
		"portCount":   c.portCount,
		"packetHex":   hex.EncodeToString(packet),
		"packetLen":   len(packet),
	}).Info("📦 注册包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送注册包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 注册包发送成功")
	return nil
}

// SendHeartbeat 发送心跳包（21指令）
func (c *TestClient) SendHeartbeat() error {
	c.logger.GetLogger().Debug("💓 发送心跳包（0x21指令）...")

	// 构建心跳包数据
	data := make([]byte, 5+c.portCount)

	// 电压（2字节，小端序）- 模拟220V
	binary.LittleEndian.PutUint16(data[0:2], 2200) // 220.0V

	// 端口数量（1字节）
	data[2] = c.portCount

	// 各端口状态（n字节）- 0=空闲
	for i := uint8(0); i < c.portCount; i++ {
		data[3+i] = 0x00 // 空闲状态
	}

	// 信号强度（1字节）- 有线组网为00
	data[3+c.portCount] = 0x00

	// 当前环境温度（1字节）- 模拟25度，需要加65
	data[4+c.portCount] = 65 + 25

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.physicalID, c.getNextMessageID(), dny_protocol.CmdDeviceHeart, data)

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送心跳包失败")
		return err
	}

	c.logger.GetLogger().WithFields(logrus.Fields{
		"voltage":     "220.0V",
		"portCount":   c.portCount,
		"temperature": "25°C",
	}).Debug("✅ 心跳包发送成功")

	return nil
}

// SendSwipeCard 发送刷卡操作（02指令）
func (c *TestClient) SendSwipeCard(cardID uint32, portNumber uint8) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"cardID":     fmt.Sprintf("0x%08X", cardID),
		"portNumber": portNumber,
	}).Info("📤 发送刷卡操作（0x02指令）...")

	// 构建刷卡数据
	data := make([]byte, 13)

	// 卡片ID（4字节，小端序）
	binary.LittleEndian.PutUint32(data[0:4], cardID)

	// 卡片类型（1字节）- 0=旧卡
	data[4] = 0x00

	// 端口号（1字节）
	data[5] = portNumber

	// 余额卡内金额（2字节，小端序）- 0表示非余额卡
	binary.LittleEndian.PutUint16(data[6:8], 0)

	// 时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[8:12], uint32(time.Now().Unix()))

	// 卡号2字节数（1字节）- 0表示无额外卡号
	data[12] = 0x00

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.physicalID, c.getNextMessageID(), dny_protocol.CmdSwipeCard, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"packetHex": hex.EncodeToString(packet),
		"packetLen": len(packet),
	}).Info("📦 刷卡包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送刷卡包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 刷卡包发送成功")
	return nil
}

// SendSettlement 发送结算信息（03指令）
func (c *TestClient) SendSettlement(portNumber uint8, chargeDuration uint16, maxPower uint16, energyConsumed uint16) error {
	c.logger.GetLogger().WithFields(logrus.Fields{
		"portNumber":     portNumber,
		"chargeDuration": chargeDuration,
		"maxPower":       maxPower,
		"energyConsumed": energyConsumed,
	}).Info("📤 发送结算信息（0x03指令）...")

	// 构建结算数据（35字节）
	data := make([]byte, 35)

	// 充电时长（2字节，小端序）
	binary.LittleEndian.PutUint16(data[0:2], chargeDuration)

	// 最大功率（2字节，小端序）
	binary.LittleEndian.PutUint16(data[2:4], maxPower)

	// 耗电量（2字节，小端序）
	binary.LittleEndian.PutUint16(data[4:6], energyConsumed)

	// 端口号（1字节）
	data[6] = portNumber

	// 在线/离线启动（1字节）- 1=在线启动
	data[7] = 0x01

	// 卡号/验证码（4字节）- 在线启动时为全0
	binary.LittleEndian.PutUint32(data[8:12], 0)

	// 停止原因（1字节）- 1=充满自停
	data[12] = 0x01

	// 订单编号（16字节）- 模拟订单号
	orderNumber := "TEST_ORDER_001234"
	copy(data[13:29], []byte(orderNumber))

	// 第二最大功率（2字节，小端序）
	binary.LittleEndian.PutUint16(data[29:31], maxPower)

	// 时间戳（4字节，小端序）
	binary.LittleEndian.PutUint32(data[31:35], uint32(time.Now().Unix()))

	// 使用已有的包构建函数
	packet := pkg.Protocol.BuildDNYResponsePacket(c.physicalID, c.getNextMessageID(), dny_protocol.CmdSettlement, data)

	c.logger.GetLogger().WithFields(logrus.Fields{
		"packetHex": hex.EncodeToString(packet),
		"packetLen": len(packet),
	}).Info("📦 结算包详情")

	// 发送数据包
	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送结算包失败")
		return err
	}

	c.logger.GetLogger().Info("✅ 结算包发送成功")
	return nil
}

// HandleServerMessages 处理服务器消息
func (c *TestClient) HandleServerMessages() {
	c.logger.GetLogger().Info("🎧 开始监听服务器消息...")

	buffer := make([]byte, 1024)

	for c.isRunning {
		// 设置读取超时
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		n, err := c.conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时继续循环
			}
			if c.isRunning {
				c.logger.GetLogger().WithError(err).Error("❌ 读取服务器消息失败")
			}
			break
		}

		if n > 0 {
			receivedData := buffer[:n]
			// 打印解析的数据
			c.logger.GetLogger().WithFields(logrus.Fields{
				"dataLen":    n,
				"dataHex":    hex.EncodeToString(receivedData),
				"dataStr":    string(receivedData),
				"remoteAddr": c.conn.RemoteAddr().String(),
				"localAddr":  c.conn.LocalAddr().String(),
				"timestamp":  time.Now().Format(time.RFC3339),
				"messageID":  c.getNextMessageID(),
				"physicalID": fmt.Sprintf("0x%08X", c.physicalID),
			}).Info("📥 收到服务器数据")

			// 使用已有的解析函数
			if pkg.Protocol.IsDNYProtocolData(receivedData) {
				c.handleDNYMessage(receivedData)
			} else {
				c.logger.GetLogger().WithFields(logrus.Fields{
					"dataStr": string(receivedData),
				}).Info("📥 收到非DNY协议数据")
			}
		}
	}
}

// handleDNYMessage 处理DNY协议消息
func (c *TestClient) handleDNYMessage(data []byte) {
	// 使用已有的解析函数
	result, err := pkg.Protocol.ParseDNYData(data)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 解析DNY消息失败")
		return
	}

	c.logger.GetLogger().WithFields(logrus.Fields{
		"command":     fmt.Sprintf("0x%02X", result.Command),
		"commandName": result.CommandName,
		"physicalID":  fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":   result.MessageID,
		"dataLen":     len(result.Data),
		"checksumOK":  result.ChecksumValid,
	}).Info("📋 解析DNY消息")

	// 根据命令类型进行处理
	switch result.Command {
	case dny_protocol.CmdDeviceRegister:
		c.handleRegisterResponse(result)
	case dny_protocol.CmdDeviceHeart:
		c.handleHeartbeatResponse(result)
	case dny_protocol.CmdNetworkStatus:
		c.handleNetworkStatusQuery(result)
	case dny_protocol.CmdChargeControl:
		c.handleChargeControl(result)
	case dny_protocol.CmdSwipeCard:
		c.handleSwipeCardResponse(result)
	case dny_protocol.CmdSettlement:
		c.handleSettlementResponse(result)
	default:
		c.logger.GetLogger().WithFields(logrus.Fields{
			"command": fmt.Sprintf("0x%02X", result.Command),
		}).Info("📋 收到未处理的指令，仅打印信息")
	}
}

// handleRegisterResponse 处理注册响应
func (c *TestClient) handleRegisterResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 1 {
		response := result.Data[0]
		if response == 0x00 {
			c.logger.GetLogger().Info("✅ 设备注册成功")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"response": fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 设备注册失败")
		}
	}
}

// handleHeartbeatResponse 处理心跳响应
func (c *TestClient) handleHeartbeatResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 1 {
		response := result.Data[0]
		if response == 0x00 || response == 0x81 {
			c.logger.GetLogger().Debug("💓 心跳响应正常")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"response": fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 心跳响应异常")
		}
	}
}

// handleNetworkStatusQuery 处理网络状态查询（81指令）
func (c *TestClient) handleNetworkStatusQuery(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到网络状态查询指令，发送注册包和心跳包")

	// 发送注册包响应
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.SendRegister()
		time.Sleep(500 * time.Millisecond)
		c.SendHeartbeat()
	}()
}

// handleChargeControl 处理充电控制指令（82指令）
func (c *TestClient) handleChargeControl(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到充电控制指令，开始解析...")

	if len(result.Data) < 30 {
		c.logger.GetLogger().Error("❌ 充电控制指令数据长度不足")
		return
	}

	// 解析充电控制数据
	rateMode := result.Data[0]
	balance := binary.LittleEndian.Uint32(result.Data[1:5])
	portNumber := result.Data[5]
	chargeCommand := result.Data[6]
	chargeDuration := binary.LittleEndian.Uint16(result.Data[7:9])
	orderNumber := string(result.Data[9:25])

	c.logger.GetLogger().WithFields(logrus.Fields{
		"rateMode":       rateMode,
		"balance":        balance,
		"portNumber":     portNumber,
		"chargeCommand":  chargeCommand,
		"chargeDuration": chargeDuration,
		"orderNumber":    orderNumber,
	}).Info("📋 充电控制指令详情")

	// 发送充电控制响应
	c.sendChargeControlResponse(result.MessageID, portNumber, orderNumber)
}

// sendChargeControlResponse 发送充电控制响应
func (c *TestClient) sendChargeControlResponse(messageID uint16, portNumber uint8, orderNumber string) {
	c.logger.GetLogger().Info("📤 发送充电控制响应...")

	// 构建响应数据
	data := make([]byte, 20)

	// 应答（1字节）- 0=执行成功
	data[0] = 0x00

	// 订单编号（16字节）
	copy(data[1:17], []byte(orderNumber))

	// 端口号（1字节）
	data[17] = portNumber

	// 待充端口（2字节）- 0表示无待充端口
	binary.LittleEndian.PutUint16(data[18:20], 0)

	// 使用原消息ID发送响应
	packet := pkg.Protocol.BuildDNYResponsePacket(c.physicalID, messageID, dny_protocol.CmdChargeControl, data)

	_, err := c.conn.Write(packet)
	if err != nil {
		c.logger.GetLogger().WithError(err).Error("❌ 发送充电控制响应失败")
		return
	}

	c.logger.GetLogger().Info("✅ 充电控制响应发送成功")
}

// handleSwipeCardResponse 处理刷卡响应
func (c *TestClient) handleSwipeCardResponse(result *protocol.DNYParseResult) {
	c.logger.GetLogger().Info("📋 收到刷卡响应")
	// 这里只是打印，实际应用中可能需要更多处理
}

// handleSettlementResponse 处理结算响应
func (c *TestClient) handleSettlementResponse(result *protocol.DNYParseResult) {
	if len(result.Data) >= 21 { // 需要至少21字节：订单号(20) + 状态(1)
		// 提取订单号 (前20字节)
		orderNumber := string(bytes.TrimRight(result.Data[0:20], "\x00"))

		// 提取状态码 (第21字节)
		response := result.Data[20]

		if response == 0x00 {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"orderNumber": orderNumber,
			}).Info("✅ 结算信息上传成功")
		} else {
			c.logger.GetLogger().WithFields(logrus.Fields{
				"orderNumber": orderNumber,
				"response":    fmt.Sprintf("0x%02X", response),
			}).Warn("⚠️ 结算信息上传失败")
		}
	} else {
		c.logger.GetLogger().WithFields(logrus.Fields{
			"dataLen": len(result.Data),
		}).Error("❌ 结算响应数据长度不足")
	}
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

// StartHeartbeat 启动心跳协程
func (c *TestClient) StartHeartbeat() {
	c.logger.GetLogger().Info("💓 启动心跳协程，间隔60秒...")

	go func() {
		ticker := time.NewTicker(60 * time.Second) // 修改为60秒，确保在服务器180秒超时前有足够的心跳包
		defer ticker.Stop()

		for c.isRunning {
			select {
			case <-ticker.C:
				if err := c.SendHeartbeat(); err != nil {
					c.logger.GetLogger().WithError(err).Error("❌ 心跳发送失败")
				}
			}
		}
	}()
}

// Stop 停止客户端
func (c *TestClient) Stop() {
	c.logger.GetLogger().Info("🛑 停止客户端...")

	c.isRunning = false
	if c.conn != nil {
		c.conn.Close()
	}

	c.logger.GetLogger().Info("✅ 客户端已停止")
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

func main() {
	fmt.Println("🚀 DNY协议完整测试客户端启动")
	fmt.Println("=====================================")

	// 初始化包依赖
	pkg.InitPackages()

	// 设置日志系统
	improvedLogger := logger.NewImprovedLogger()
	improvedLogger.GetLogger().SetLevel(logrus.InfoLevel)
	improvedLogger.GetLogger().SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		ForceColors:     true,
	})

	// 创建测试客户端
	client := NewTestClient()
	client.logger = improvedLogger

	// 显示客户端配置
	client.logger.GetLogger().WithFields(logrus.Fields{
		"physicalID":  fmt.Sprintf("0x%08X", client.physicalID),
		"deviceType":  fmt.Sprintf("0x%02X (新款485双模)", client.deviceType),
		"portCount":   client.portCount,
		"firmwareVer": fmt.Sprintf("V%d.%02d", client.firmwareVer/100, client.firmwareVer%100),
	}).Info("🔧 客户端配置")

	// 连接服务器
	serverAddr := "localhost:7054"
	if err := client.Connect(serverAddr); err != nil {
		client.logger.GetLogger().WithError(err).Fatal("❌ 连接服务器失败")
	}
	defer client.Stop()

	// 设置运行状态
	client.isRunning = true

	// 启动消息处理协程
	go client.HandleServerMessages()

	// 发送ICCID
	if err := client.SendICCID(); err != nil {
		client.logger.GetLogger().WithError(err).Fatal("❌ 发送ICCID失败")
	}
	time.Sleep(1 * time.Second)

	// 发送设备注册包
	if err := client.SendRegister(); err != nil {
		client.logger.GetLogger().WithError(err).Fatal("❌ 发送注册包失败")
	}
	time.Sleep(2 * time.Second)

	// 启动心跳
	client.StartHeartbeat()

	// 运行测试序列
	go func() {
		time.Sleep(5 * time.Second) // 等待注册完成
		if err := client.RunTestSequence(); err != nil {
			client.logger.GetLogger().WithError(err).Error("❌ 测试序列执行失败")
		}
	}()

	// 设置信号处理，支持优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	client.logger.GetLogger().Info("🎯 客户端开始持续运行，按 Ctrl+C 退出...")
	client.logger.GetLogger().Info("💡 支持的退出信号: SIGINT (Ctrl+C), SIGTERM")

	// 等待退出信号
	sig := <-sigChan
	client.logger.GetLogger().WithFields(logrus.Fields{
		"signal": sig.String(),
	}).Info("🔔 收到退出信号，开始优雅关闭...")

	client.logger.GetLogger().Info("🏁 程序退出")
}
