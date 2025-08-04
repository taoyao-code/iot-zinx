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
	*BaseHandler
	simCardHandler    *SimCardHandler
	deviceRegister    *DeviceRegisterRouter
	heartbeat         *HeartbeatRouter
	charging          *ChargingRouter
	connectionMonitor *ConnectionMonitor
}

// NewUnifiedDataHandler 创建统一数据处理器
func NewUnifiedDataHandler() *UnifiedDataHandler {
	return &UnifiedDataHandler{
		BaseHandler:    NewBaseHandler("UnifiedDataHandler"),
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
		// 使用统一的协议解析和验证
		parsedMsg, err := h.ParseAndValidateMessage(request)
		if err != nil {
			// 检查是否是未知消息类型错误，如果是则降级为WARN
			if strings.Contains(err.Error(), "unknown message type") {
				logger.Warn("UnifiedDataHandler: 收到未知消息类型",
					zap.Uint64("connID", conn.GetConnID()),
					zap.String("error", err.Error()),
					zap.String("dataHex", fmt.Sprintf("%x", data)),
				)
				// 对于未知消息类型，尝试使用通用处理
				h.handleUnknownMessage(request, data)
				return
			}

			logger.Error("UnifiedDataHandler: DNY协议解析失败",
				zap.Uint64("connID", conn.GetConnID()),
				zap.Error(err),
			)
			return
		}

		// 设备必须先通过0x20注册包正式注册，不允许自动注册

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

		case dny_protocol.MsgTypeExtendedCommand:
			logger.Info("UnifiedDataHandler: 收到扩展命令数据包(0x05)",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
			)
			h.handleExtendedMessage(request, parsedMsg)

		case dny_protocol.MsgTypeExtHeartbeat1, dny_protocol.MsgTypeExtHeartbeat2, dny_protocol.MsgTypeExtHeartbeat3,
			dny_protocol.MsgTypeExtHeartbeat4, dny_protocol.MsgTypeExtHeartbeat5, dny_protocol.MsgTypeExtHeartbeat6,
			dny_protocol.MsgTypeExtHeartbeat7, dny_protocol.MsgTypeExtHeartbeat8:
			logger.Debug("UnifiedDataHandler: 收到扩展心跳数据包",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
			)
			h.handleExtendedMessage(request, parsedMsg)

		case dny_protocol.MsgTypeExtCommand1, dny_protocol.MsgTypeExtCommand2, dny_protocol.MsgTypeExtCommand3, dny_protocol.MsgTypeExtCommand4:
			logger.Info("UnifiedDataHandler: 收到扩展命令数据包",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
			)
			h.handleExtendedMessage(request, parsedMsg)

		case dny_protocol.MsgTypeExtStatus1, dny_protocol.MsgTypeExtStatus2, dny_protocol.MsgTypeExtStatus3,
			dny_protocol.MsgTypeExtStatus4, dny_protocol.MsgTypeExtStatus5, dny_protocol.MsgTypeExtStatus6,
			dny_protocol.MsgTypeExtStatus7, dny_protocol.MsgTypeExtStatus8, dny_protocol.MsgTypeExtStatus9,
			dny_protocol.MsgTypeExtStatus10, dny_protocol.MsgTypeExtStatus11, dny_protocol.MsgTypeExtStatus12,
			dny_protocol.MsgTypeExtStatus13, dny_protocol.MsgTypeExtStatus14, dny_protocol.MsgTypeExtStatus15,
			dny_protocol.MsgTypeExtStatus16, dny_protocol.MsgTypeExtStatus17, dny_protocol.MsgTypeExtStatus18,
			dny_protocol.MsgTypeExtStatus19, dny_protocol.MsgTypeExtStatus20:
			logger.Debug("UnifiedDataHandler: 收到扩展状态数据包",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
			)
			h.handleExtendedMessage(request, parsedMsg)

		case dny_protocol.MsgTypeNewType:
			logger.Info("UnifiedDataHandler: 收到新类型数据包(0xF1)",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.Int("dataLen", len(parsedMsg.Data.([]byte))),
			)
			// TODO: 实现0xF1类型处理逻辑

		default:
			logger.Debug("UnifiedDataHandler: 收到其他类型数据包",
				zap.Uint64("connID", conn.GetConnID()),
				zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
				zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
			)
			h.handleExtendedMessage(request, parsedMsg)
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

// handleUnknownMessage 处理未知消息类型
func (h *UnifiedDataHandler) handleUnknownMessage(request ziface.IRequest, data []byte) {
	conn := request.GetConnection()

	logger.Debug("UnifiedDataHandler: 处理未知消息类型",
		zap.Uint64("connID", conn.GetConnID()),
		zap.String("dataHex", fmt.Sprintf("%x", data)),
		zap.Int("dataLen", len(data)),
	)

	// 对于未知消息，暂时不做特殊处理，只记录日志
	// 未来可以在这里添加通用的响应逻辑
}

// handleExtendedMessage 处理扩展消息类型
func (h *UnifiedDataHandler) handleExtendedMessage(request ziface.IRequest, parsedMsg *dny_protocol.ParsedMessage) {
	conn := request.GetConnection()

	// 获取扩展消息数据
	extData, ok := parsedMsg.Data.(*dny_protocol.ExtendedMessageData)
	if !ok {
		logger.Warn("UnifiedDataHandler: 扩展消息数据类型转换失败",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
		)
		return
	}

	// 根据消息类别进行处理
	category := extData.GetMessageCategory()

	logger.Debug("UnifiedDataHandler: 处理扩展消息",
		zap.Uint64("connID", conn.GetConnID()),
		zap.String("command", fmt.Sprintf("0x%02x", parsedMsg.Command)),
		zap.String("category", category),
		zap.Int("dataLen", extData.DataLength),
		zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
	)

	switch category {
	case "extended_heartbeat":
		// 扩展心跳包处理 - 可以考虑转发给心跳处理器
		logger.Debug("处理扩展心跳包",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
		)
		// TODO: 可以在这里添加心跳包的统计和监控逻辑

	case "extended_status":
		// 扩展状态包处理
		logger.Debug("处理扩展状态包",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
		)
		// TODO: 可以在这里添加状态监控和第三方平台通知逻辑

	case "extended_command":
		// 扩展命令包处理
		logger.Debug("处理扩展命令包",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
		)
		// TODO: 可以在这里添加命令响应逻辑

	default:
		logger.Debug("处理未分类扩展消息",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("messageType", dny_protocol.GetMessageTypeName(parsedMsg.MessageType)),
		)
	}
}
