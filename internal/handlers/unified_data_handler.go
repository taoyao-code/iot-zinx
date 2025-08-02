package handlers

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"go.uber.org/zap"
)

// UnifiedDataHandler 统一数据处理器
// 负责分发不同类型的数据包到对应的专门处理器
type UnifiedDataHandler struct {
	znet.BaseRouter
	simCardHandler    *SimCardHandler
	deviceRegister    *DeviceRegisterRouter
	heartbeat         *HeartbeatRouter
	charging          *ChargingRouter
	connectionMonitor *ConnectionMonitor
}

// NewUnifiedDataHandler 创建统一数据处理器
func NewUnifiedDataHandler() *UnifiedDataHandler {
	return &UnifiedDataHandler{
		simCardHandler: &SimCardHandler{},
		deviceRegister: NewDeviceRegisterRouter(),
		heartbeat:      NewHeartbeatRouter(),
		charging:       NewChargingRouter(),
	}
}

// SetConnectionMonitor 设置连接监控器
func (h *UnifiedDataHandler) SetConnectionMonitor(monitor *ConnectionMonitor) {
	h.connectionMonitor = monitor
	h.deviceRegister.SetConnectionMonitor(monitor)
	h.heartbeat.SetConnectionMonitor(monitor)
}

// Handle 统一处理所有传入的数据包
func (h *UnifiedDataHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 强制调试输出
	fmt.Printf("🔥 UnifiedDataHandler: connID=%d, dataLen=%d, dataHex=%x, dataStr=%s\n",
		conn.GetConnID(), len(data), data, string(data))

	logger.Info("UnifiedDataHandler: 收到数据包",
		zap.Uint64("connID", conn.GetConnID()),
		zap.String("remoteAddr", conn.RemoteAddr().String()),
		zap.Int("dataLen", len(data)),
		zap.String("dataHex", fmt.Sprintf("%x", data)),
	)

	// 判断数据包类型并分发
	packetType := h.identifyPacketType(data)

	switch packetType {
	case "iccid":
		logger.Info("UnifiedDataHandler: 分发ICCID数据包到SimCardHandler",
			zap.Uint64("connID", conn.GetConnID()),
		)
		h.simCardHandler.Handle(request)

	case "dny":
		// 解析DNY协议包
		parsedMsg := dny_protocol.ParseDNYMessage(data)
		if err := dny_protocol.ValidateMessage(parsedMsg); err != nil {
			logger.Error("UnifiedDataHandler: DNY协议解析失败",
				zap.Uint64("connID", conn.GetConnID()),
				zap.Error(err),
			)
			return
		}

		// 根据DNY命令分发
		switch parsedMsg.MessageType {
		case dny_protocol.MsgTypeOldHeartbeat:
			logger.Info("UnifiedDataHandler: 分发旧版心跳包到HeartbeatRouter",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			h.heartbeat.Handle(request)

		case dny_protocol.MsgTypeDeviceRegister:
			logger.Info("UnifiedDataHandler: 分发设备注册包到DeviceRegisterRouter",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			h.deviceRegister.Handle(request)

		case dny_protocol.MsgTypeHeartbeat:
			logger.Info("UnifiedDataHandler: 分发心跳包到HeartbeatRouter",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			h.heartbeat.Handle(request)

		case dny_protocol.MsgTypeSwipeCard:
			logger.Info("UnifiedDataHandler: 分发刷卡请求到对应处理器",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			// TODO: 实现刷卡处理逻辑
			logger.Info("刷卡请求处理", zap.Any("data", parsedMsg.Data))

		case dny_protocol.MsgTypeSettlement:
			logger.Info("UnifiedDataHandler: 分发结算数据到对应处理器",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			// TODO: 实现结算数据处理逻辑
			logger.Info("结算数据处理", zap.Any("data", parsedMsg.Data))

		case dny_protocol.MsgTypeOrderConfirm:
			logger.Info("UnifiedDataHandler: 分发订单确认到对应处理器",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			// TODO: 实现订单确认处理逻辑
			logger.Info("订单确认处理", zap.Any("data", parsedMsg.Data))

		case dny_protocol.MsgTypePowerHeartbeat:
			logger.Info("UnifiedDataHandler: 分发功率心跳包到对应处理器",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			// TODO: 实现功率心跳处理逻辑
			logger.Info("功率心跳处理", zap.Any("data", parsedMsg.Data))

		case dny_protocol.MsgTypeChargeControl:
			logger.Info("UnifiedDataHandler: 分发充电控制包到ChargingRouter",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			h.charging.Handle(request)

		case dny_protocol.MsgTypeNewType:
			logger.Info("UnifiedDataHandler: 收到新类型数据包(0xF1)",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.Int("dataLen", len(parsedMsg.Data.([]byte))),
			)
			// TODO: 实现0xF1类型处理逻辑

		default:
			logger.Warn("UnifiedDataHandler: 未知的DNY命令类型",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.String("messageType", string(parsedMsg.MessageType)),
			)
		}

	case "link":
		logger.Info("UnifiedDataHandler: 收到Link心跳包",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("content", string(data)),
		)
		// Link心跳包暂时不处理，只记录

	default:
		logger.Warn("UnifiedDataHandler: 未知数据包类型",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("dataHex", fmt.Sprintf("%x", data)),
			zap.String("dataStr", string(data)),
		)
	}
}

// identifyPacketType 识别数据包类型
func (h *UnifiedDataHandler) identifyPacketType(data []byte) string {
	// 1. 检查是否为ICCID包
	if len(data) == constants.IotSimCardLength && h.isValidICCID(data) {
		return "iccid"
	}

	// 2. 检查是否为Link心跳包
	if len(data) == constants.LinkMessageLength && string(data) == constants.LinkMessagePayload {
		return "link"
	}

	// 3. 检查是否为DNY协议包 - 修复短包判断
	if len(data) >= 9 && string(data[:3]) == constants.ProtocolHeader {
		// 9字节是DNY协议的最小长度：DNY(3) + Length(2) + PhysicalID(4)
		// 进一步验证Length字段的合理性
		if len(data) >= 5 {
			length := uint16(data[3]) | uint16(data[4])<<8 // 小端序读取Length
			expectedTotal := 5 + int(length)               // DNY(3) + Length(2) + Length内容

			// 对于长度不匹配但格式正确的包，仍然尝试解析
			if expectedTotal <= len(data)+10 { // 允许10字节的容差
				return "dny"
			}
		}

		// 如果Length字段异常，但确实是DNY开头，仍然尝试解析
		return "dny"
	}

	return "unknown"
}

// isValidICCID 验证ICCID格式
func (h *UnifiedDataHandler) isValidICCID(data []byte) bool {
	if len(data) != constants.IotSimCardLength {
		return false
	}

	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头
	if !strings.HasPrefix(dataStr, constants.ICCIDValidPrefix) {
		return false
	}

	// 必须全部为十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}
